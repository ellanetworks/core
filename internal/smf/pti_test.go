// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func buildPDUSessionEstRequestWithPTI(pti uint8) []byte {
	ipv4 := fgs.PDUSessionTypeIPv4

	req := &fgs.PDUSessionEstablishmentRequest{
		PDUSessionID:             1,
		PTI:                      pti,
		IntegrityProtMaxDataRate: [2]byte{0xff, 0xff},
		PDUSessionType:           &ipv4,
	}

	buf, err := req.Marshal()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Establishment Request: %v", err))
	}

	return buf
}

func buildPDUSessionEstRequestAlwaysOn() []byte {
	ipv4 := fgs.PDUSessionTypeIPv4

	req := &fgs.PDUSessionEstablishmentRequest{
		PDUSessionID:             1,
		PTI:                      10,
		IntegrityProtMaxDataRate: [2]byte{0xff, 0xff},
		PDUSessionType:           &ipv4,
		AlwaysOnRequested:        true,
	}

	buf, err := req.Marshal()
	if err != nil {
		panic(fmt.Sprintf("build PDU Session Establishment Request (always-on): %v", err))
	}

	return buf
}

func buildPDUSessionReleaseComplete(pduSessionID, pti uint8) []byte {
	return []byte{fgs.EPD5GSM, pduSessionID, pti, uint8(fgs.MsgPDUSessionReleaseComplete)}
}

func status5GSMCause(t *testing.T, raw []byte) uint8 {
	t.Helper()

	m, err := fgs.ParseStatus5GSM(raw)
	if err != nil {
		t.Fatalf("decode 5GSM STATUS: %v", err)
	}

	return m.Cause
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

	if got := status5GSMCause(t, rsp.N1Msg); got != fgs.Cause5GSMPTIMismatch {
		t.Errorf("STATUS cause = %d, want %d (#47 PTI mismatch)", got, fgs.Cause5GSMPTIMismatch)
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

	if got := status5GSMCause(t, rsp.N1Msg); got != fgs.Cause5GSMInvalidPTIValue {
		t.Errorf("STATUS cause = %d, want %d (#81 invalid PTI value)", got, fgs.Cause5GSMInvalidPTIValue)
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

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(calls[0].n1Msg)
	if err != nil {
		t.Fatalf("decode Accept: %v", err)
	}

	if acc.AlwaysOn == nil {
		t.Fatal("UE requested always-on; TS 24.501 §6.4.1 (case b-i) requires an Always-on PDU session indication in the Accept, got none")
	}

	if *acc.AlwaysOn != 0 {
		t.Errorf("APSI = %d, want 0 (always-on not allowed)", *acc.AlwaysOn)
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

	if got := status5GSMCause(t, rsp); got != fgs.Cause5GSMInvalidPTIValue {
		t.Errorf("STATUS cause = %d, want %d (#81 invalid PTI value)", got, fgs.Cause5GSMInvalidPTIValue)
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
