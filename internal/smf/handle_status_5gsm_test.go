// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func build5GSMStatus(pduSessionID, pti, cause uint8) []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypeStatus5GSM)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.Status5GSM = nasMessage.NewStatus5GSM(0)
	m.Status5GSM.SetMessageType(nas.MsgTypeStatus5GSM)
	m.Status5GSM.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.Status5GSM.SetPDUSessionID(pduSessionID)
	m.Status5GSM.SetPTI(pti)
	m.Status5GSM.SetCauseValue(cause)

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build 5GSM STATUS: %v", err))
	}

	return buf
}

// TS 24.501 §6.5.3.
func TestStatus5GSM_InvalidPDUSessionIdentityReleasesLocally(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, build5GSMStatus(smCtx.PDUSessionID, 0, nasMessage.Cause5GSMInvalidPDUSessionIdentity)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("PDU session retained after 5GSM STATUS #43, want it released")
	}
}

// TS 24.501 §6.5.3: a #47 naming the establishment accept's PTI means the UE never took
// the session.
func TestStatus5GSM_PTIMismatchOnEstablishmentAcceptReleasesLocally(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, rsp, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, buildPDUSessionEstRequestWithPTI(10))
	if err != nil {
		t.Fatalf("CreateSmContext failed: %v", err)
	}

	if rsp != nil || ref == "" {
		t.Fatalf("expected a successful establishment, got ref %q with a %d-byte reject", ref, len(rsp))
	}

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, build5GSMStatus(1, 10, nasMessage.Cause5GSMPTIMismatch)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("PDU session retained after 5GSM STATUS #47 named the establishment accept's PTI, want it released")
	}
}

// TS 24.501 §6.5.3: #47 releases only when it names the establishment accept's PTI.
func TestStatus5GSM_PTIMismatchOnAnotherPTIKeepsSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, buildPDUSessionEstRequestWithPTI(10))
	if err != nil {
		t.Fatalf("CreateSmContext failed: %v", err)
	}

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, build5GSMStatus(1, 9, nasMessage.Cause5GSMPTIMismatch)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("PDU session released by a 5GSM STATUS #47 naming an unrelated PTI, want it retained")
	}
}

// The user plane is freed when the release starts (TS 23.502 §4.3.4.2 step 2) and no
// reconcile sweep re-derives a UE-requested release, so a release aborted by a 5GSM
// STATUS is completed here or never.
func TestStatus5GSM_AbortingAnInFlightReleaseRemovesSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseRequest(smCtx.PDUSessionID, 5)); err != nil {
		t.Fatalf("release request: %v", err)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("session removed before the UE answered the release command")
	}

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, build5GSMStatus(smCtx.PDUSessionID, 5, nasMessage.Cause5GSMMessageTypeNonExistentOrNotImplemented)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("PDU session retained after a 5GSM STATUS aborted its release, want it released")
	}
}

// The previous configuration is kept so the reconcile sweep re-derives the modification
// (TS 24.501 §6.5.3: the local action for a cause the clause does not name is
// implementation dependent).
func TestStatus5GSM_UnrelatedCauseKeepsSessionAndDiscardsPendingPolicy(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, ref) // new AMBR 500/600 Mbps, held pending

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, build5GSMStatus(smCtx.PDUSessionID, 0, nasMessage.Cause5GSMProtocolErrorUnspecified)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("PDU session released by a 5GSM STATUS with no release in flight, want it retained")
	}

	smCtx.Mutex.Lock()
	dl := smCtx.PolicyData.Ambr.Downlink
	smCtx.Mutex.Unlock()

	if dl != "200 Mbps" {
		t.Fatalf("AMBR downlink = %q after a 5GSM STATUS aborted the modification, want \"200 Mbps\"", dl)
	}
}

// TS 24.501 §7.3.1 d); clause 7 applies ahead of the §6.5.3 cause handling, so even #43
// takes no action.
func TestStatus5GSM_ReservedPTIIgnored(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, build5GSMStatus(smCtx.PDUSessionID, 0xff, nasMessage.Cause5GSMInvalidPDUSessionIdentity)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("PDU session released by a 5GSM STATUS with a reserved PTI, want it retained")
	}
}
