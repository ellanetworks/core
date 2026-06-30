// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

func buildPDUSessionEstRequestWithPTI(pti uint8) []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentRequest)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentRequest = nasMessage.NewPDUSessionEstablishmentRequest(0)
	m.PDUSessionEstablishmentRequest.SetMessageType(nas.MsgTypePDUSessionEstablishmentRequest)
	m.PDUSessionEstablishmentRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentRequest.SetPDUSessionID(1)
	m.PDUSessionEstablishmentRequest.SetPTI(pti)
	m.PDUSessionEstablishmentRequest.IntegrityProtectionMaximumDataRate. //nolint:staticcheck // full path needed to avoid ambiguous selector
										SetMaximumDataRatePerUEForUserPlaneIntegrityProtectionForUpLink(0xff)
	m.PDUSessionEstablishmentRequest.IntegrityProtectionMaximumDataRate. //nolint:staticcheck // full path needed to avoid ambiguous selector
										SetMaximumDataRatePerUEForUserPlaneIntegrityProtectionForDownLink(0xff)
	m.PDUSessionEstablishmentRequest.PDUSessionType = nasType.NewPDUSessionType( //nolint:staticcheck // full path needed to avoid ambiguous selector
		nasMessage.PDUSessionEstablishmentRequestPDUSessionTypeType)
	m.PDUSessionEstablishmentRequest.PDUSessionType.SetPDUSessionTypeValue(nasMessage.PDUSessionTypeIPv4) //nolint:staticcheck // full path needed to avoid ambiguous selector

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Establishment Request: %v", err))
	}

	return buf
}

func buildPDUSessionEstRequestAlwaysOn() []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentRequest)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentRequest = nasMessage.NewPDUSessionEstablishmentRequest(0)
	m.PDUSessionEstablishmentRequest.SetMessageType(nas.MsgTypePDUSessionEstablishmentRequest)
	m.PDUSessionEstablishmentRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentRequest.SetPDUSessionID(1)
	m.PDUSessionEstablishmentRequest.SetPTI(10)
	m.PDUSessionEstablishmentRequest.IntegrityProtectionMaximumDataRate. //nolint:staticcheck // full path needed to avoid ambiguous selector
										SetMaximumDataRatePerUEForUserPlaneIntegrityProtectionForUpLink(0xff)
	m.PDUSessionEstablishmentRequest.IntegrityProtectionMaximumDataRate. //nolint:staticcheck // full path needed to avoid ambiguous selector
										SetMaximumDataRatePerUEForUserPlaneIntegrityProtectionForDownLink(0xff)
	m.PDUSessionEstablishmentRequest.PDUSessionType = nasType.NewPDUSessionType( //nolint:staticcheck // full path needed to avoid ambiguous selector
		nasMessage.PDUSessionEstablishmentRequestPDUSessionTypeType)
	m.PDUSessionEstablishmentRequest.PDUSessionType.SetPDUSessionTypeValue(nasMessage.PDUSessionTypeIPv4) //nolint:staticcheck // full path needed to avoid ambiguous selector
	m.PDUSessionEstablishmentRequest.AlwaysonPDUSessionRequested = nasType.NewAlwaysonPDUSessionRequested(
		nasMessage.PDUSessionEstablishmentRequestAlwaysonPDUSessionRequestedType)
	m.PDUSessionEstablishmentRequest.SetAPSR(1)

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Establishment Request (always-on): %v", err))
	}

	return buf
}

func buildPDUSessionReleaseComplete(pduSessionID, pti uint8) []byte {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionReleaseComplete)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseComplete = nasMessage.NewPDUSessionReleaseComplete(0)
	m.PDUSessionReleaseComplete.SetMessageType(nas.MsgTypePDUSessionReleaseComplete)
	m.PDUSessionReleaseComplete.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseComplete.SetPDUSessionID(pduSessionID)
	m.PDUSessionReleaseComplete.SetPTI(pti)

	buf, err := m.PlainNasEncode()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Release Complete: %v", err))
	}

	return buf
}

func status5GSMCause(t *testing.T, raw []byte) uint8 {
	t.Helper()

	m := new(nas.Message)
	if err := m.PlainNasDecode(&raw); err != nil {
		t.Fatalf("decode 5GSM STATUS: %v", err)
	}

	if m.Status5GSM == nil {
		t.Fatalf("expected 5GSM STATUS, got message type %d", m.GsmHeader.GetMessageType())
	}

	return m.Status5GSM.GetCauseValue()
}

// A PDU SESSION RELEASE COMPLETE whose PTI matches no procedure in use is
// answered with a 5GSM STATUS #47 "PTI mismatch" (TS 24.501).
func TestUpdateSmContextN1Msg_ReleaseComplete_PTIMismatch(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	n1Msg := buildPDUSessionReleaseComplete(smCtx.PDUSessionID, 5)

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, n1Msg)
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if rsp == nil || rsp.N1Msg == nil {
		t.Fatal("expected a 5GSM STATUS response (TS 24.501 §7.3.1 a), got none")
	}

	if got := status5GSMCause(t, rsp.N1Msg); got != nasMessage.Cause5GSMPTIMismatch {
		t.Errorf("STATUS cause = %d, want %d (#47 PTI mismatch)", got, nasMessage.Cause5GSMPTIMismatch)
	}
}

// A PDU SESSION RELEASE COMPLETE whose PTI matches the in-flight release
// procedure is processed, not answered with a STATUS (TS 24.501).
func TestUpdateSmContextN1Msg_ReleaseComplete_PTIInUse(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	const pti = 7

	if _, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseRequest(smCtx.PDUSessionID, pti)); err != nil {
		t.Fatalf("release request failed: %v", err)
	}

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseComplete(smCtx.PDUSessionID, pti))
	if err != nil {
		t.Fatalf("release complete failed: %v", err)
	}

	if rsp != nil && rsp.N1Msg != nil {
		t.Errorf("release complete with a matching PTI must be processed, not answered with a STATUS; got %d-byte N1 message", len(rsp.N1Msg))
	}
}

// A PDU SESSION RELEASE REQUEST with an unassigned PTI is answered with a 5GSM
// STATUS #81 and must not tear down the session (TS 24.501).
func TestUpdateSmContextN1Msg_ReleaseRequest_UnassignedPTI(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionReleaseRequest(smCtx.PDUSessionID, 0))
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if rsp == nil || rsp.N1Msg == nil {
		t.Fatal("expected a 5GSM STATUS response (TS 24.501 §7.3.1 c), got none")
	}

	if got := status5GSMCause(t, rsp.N1Msg); got != nasMessage.Cause5GSMInvalidPTIValue {
		t.Errorf("STATUS cause = %d, want %d (#81 invalid PTI value)", got, nasMessage.Cause5GSMInvalidPTIValue)
	}

	upf.mu.Lock()
	deletes := len(upf.deleteCalls)
	upf.mu.Unlock()

	if deletes != 0 {
		t.Errorf("an invalid-PTI release request must not release the tunnel; got %d DeleteSession calls", deletes)
	}
}

// A 5GSM message with a reserved PTI is ignored (TS 24.501 §7.3.1 d).
func TestUpdateSmContextN1Msg_ReservedPTI_Ignored(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	rsp, err := s.UpdateSmContextN1Msg(ctx, ref, buildPDUSessionModificationRequest(smCtx.PDUSessionID, 0xff))
	if err != nil {
		t.Fatalf("UpdateSmContextN1Msg failed: %v", err)
	}

	if rsp != nil {
		t.Errorf("a reserved-PTI message must be ignored (no response); got a response")
	}
}

// A PDU SESSION ESTABLISHMENT REQUEST carrying the Always-on PDU session
// requested IE must be answered with an Establishment Accept that includes the
// Always-on PDU session indication; Ella does not grant always-on, so the value
// is "not allowed" (APSI 0) per TS 24.501 §6.4.1 (case b-i).
func TestCreateSmContext_AlwaysOnRequested_IndicationNotAllowed(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	ref, rsp, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, buildPDUSessionEstRequestAlwaysOn())
	if err != nil {
		t.Fatalf("CreateSmContext failed: %v", err)
	}

	if rsp != nil || ref == "" {
		t.Fatalf("expected a successful establishment, got ref %q with %d-byte reject", ref, len(rsp))
	}

	amfCb.mu.Lock()
	calls := amfCb.n1n2Calls
	amfCb.mu.Unlock()

	if len(calls) != 1 {
		t.Fatalf("expected 1 N1N2 transfer (the Accept), got %d", len(calls))
	}

	m := new(nas.Message)

	n1 := calls[0].n1Msg
	if err := m.PlainNasDecode(&n1); err != nil {
		t.Fatalf("decode Accept: %v", err)
	}

	if m.PDUSessionEstablishmentAccept == nil {
		t.Fatalf("expected an Establishment Accept, got message type %d", m.GsmHeader.GetMessageType())
	}

	if m.PDUSessionEstablishmentAccept.AlwaysonPDUSessionIndication == nil {
		t.Fatal("UE requested always-on; TS 24.501 §6.4.1 (case b-i) requires an Always-on PDU session indication in the Accept, got none")
	}

	if got := m.PDUSessionEstablishmentAccept.GetAPSI(); got != 0 {
		t.Errorf("APSI = %d, want 0 (always-on not allowed)", got)
	}
}

// A PDU SESSION ESTABLISHMENT REQUEST with an unassigned PTI is answered with a
// 5GSM STATUS #81 and establishes no session (TS 24.501 §7.3.1 c).
func TestCreateSmContext_UnassignedPTI_Status81(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	ref, rsp, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, buildPDUSessionEstRequestWithPTI(0))
	if err != nil {
		t.Fatalf("CreateSmContext failed: %v", err)
	}

	if ref != "" {
		t.Errorf("an invalid-PTI establishment request must create no session; got ref %q", ref)
	}

	if rsp == nil {
		t.Fatal("expected a 5GSM STATUS response (TS 24.501 §7.3.1 c), got none")
	}

	if got := status5GSMCause(t, rsp); got != nasMessage.Cause5GSMInvalidPTIValue {
		t.Errorf("STATUS cause = %d, want %d (#81 invalid PTI value)", got, nasMessage.Cause5GSMInvalidPTIValue)
	}

	upf.mu.Lock()
	established := upf.lastEstablish
	upf.mu.Unlock()

	if established != nil {
		t.Error("an invalid-PTI establishment request must not establish a PFCP session")
	}
}

// A PDU SESSION ESTABLISHMENT REQUEST with a reserved PTI is ignored: no
// session, no response (TS 24.501 §7.3.1 d).
func TestCreateSmContext_ReservedPTI_Ignored(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()
	supi := testSUPI()

	ref, rsp, err := s.CreateSmContext(ctx, supi, 1, testDNN, testSnssai, buildPDUSessionEstRequestWithPTI(0xff))
	if err != nil {
		t.Fatalf("CreateSmContext failed: %v", err)
	}

	if ref != "" || rsp != nil {
		t.Errorf("a reserved-PTI establishment request must be ignored; got ref %q, %d-byte response", ref, len(rsp))
	}
}
