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

// TestStatus5GSM_InvalidPDUSessionIdentityReleasesLocally verifies TS 24.501 §6.5.3:
// 5GSM cause #43 makes the SMF locally release the PDU session named in the STATUS.
func TestStatus5GSM_InvalidPDUSessionIdentityReleasesLocally(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, build5GSMStatus(smCtx.PDUSessionID, 0, nasMessage.Cause5GSMInvalidPDUSessionIdentity)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("PDU session retained after 5GSM STATUS #43; TS 24.501 §6.5.3 requires a local release")
	}
}

// TestStatus5GSM_PTIMismatchOnEstablishmentAcceptReleasesLocally verifies TS 24.501
// §6.5.3: a 5GSM STATUS #47 naming the PTI of the PDU SESSION ESTABLISHMENT ACCEPT
// means the UE never took the session, so the SMF locally releases it. This is the 5G
// spelling of the 4G ACTIVATE DEFAULT EPS BEARER CONTEXT REJECT (TS 24.301 §7.3.1 g).
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
		t.Fatal("PDU session retained after 5GSM STATUS #47 named the establishment accept's PTI; the UE never took the session, so the SMF would anchor a session no UE has")
	}
}

// TestStatus5GSM_PTIMismatchOnAnotherPTIKeepsSession verifies that #47 releases the
// session only when it names the establishment accept's PTI (TS 24.501 §6.5.3); any
// other PTI just aborts that procedure.
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
		t.Fatal("PDU session released by a 5GSM STATUS #47 naming a PTI unrelated to the establishment accept")
	}
}

// TestStatus5GSM_AbortingAnInFlightReleaseRemovesSession verifies that a 5GSM STATUS
// carrying a cause other than #43 still removes the session when it aborts an in-flight
// release. The user plane is freed when the release starts (TS 23.502 §4.3.4.2 step 2)
// and no reconcile sweep re-derives a UE-requested release, so the session is removed
// here or never. Mirrors the MME's ESM STATUS handling (TS 24.301 §6.7).
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

	// The UE answers the Release Command with a STATUS instead of a Release Complete.
	if _, err := s.UpdateSmContextN1Msg(ctx, ref, build5GSMStatus(smCtx.PDUSessionID, 5, nasMessage.Cause5GSMMessageTypeNonExistentOrNotImplemented)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) != nil {
		t.Fatal("PDU session retained after a 5GSM STATUS aborted its release; its user plane is already gone and nothing retries the teardown")
	}
}

// TestStatus5GSM_UnrelatedCauseKeepsSessionAndDiscardsPendingPolicy verifies that a
// 5GSM STATUS with no release in flight leaves the session up and only abandons the
// outstanding modification, keeping the previous configuration so the reconcile sweep
// re-derives it (TS 24.501 §6.5.3: the local action for such a cause is implementation
// dependent).
func TestStatus5GSM_UnrelatedCauseKeepsSessionAndDiscardsPendingPolicy(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, ref) // new AMBR 500/600 Mbps, held pending

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, build5GSMStatus(smCtx.PDUSessionID, 0, nasMessage.Cause5GSMProtocolErrorUnspecified)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("PDU session released by a 5GSM STATUS with no release in flight")
	}

	smCtx.Mutex.Lock()
	dl := smCtx.PolicyData.Ambr.Downlink
	smCtx.Mutex.Unlock()

	if dl != "200 Mbps" {
		t.Fatalf("AMBR downlink = %q after a 5GSM STATUS aborted the modification, want the previous \"200 Mbps\"", dl)
	}
}

// TestStatus5GSM_ReservedPTIIgnored verifies TS 24.501 §7.3.1 d): a 5GSM message with a
// reserved PTI is ignored. Clause 7 applies ahead of the §6.5.3 cause handling, so even
// #43 takes no action.
func TestStatus5GSM_ReservedPTIIgnored(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, build5GSMStatus(smCtx.PDUSessionID, 0xff, nasMessage.Cause5GSMInvalidPDUSessionIdentity)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("PDU session released by a 5GSM STATUS with a reserved PTI, which must be ignored")
	}
}
