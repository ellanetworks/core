// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// TestInitialUEMessageResumeMacFailedTAURejects verifies that a resume TRACKING
// AREA UPDATE whose integrity check fails is rejected with EMM cause #9 so the UE
// re-attaches at once, rather than being silently dropped (TS 24.301 §5.5.3.2.5).
func TestInitialUEMessageResumeMacFailedTAURejects(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code := m.MmeIdentity()
	m.ReallocateGUTI(ue, plmn, group, code)
	mtmsi := ue.TmsiForTest()

	tau, err := (&eps.TrackingAreaUpdateRequest{EPSUpdateType: 3}).Marshal() // periodic
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(tau, eps.SHTIntegrityProtected, nascommon.NASCount(0, 0),
		nascommon.DirectionUplink, ue.KnasIntForTest(), ue.KnasEncForTest(),
		nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	wire[1] ^= 0xff // corrupt the MAC so the resume fails the integrity check

	plmnID := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	initialUE := &s1ap.InitialUEMessage{
		ENBUES1APID:           1001,
		NASPDU:                s1ap.NASPDU(wire),
		TAI:                   s1ap.TAI{PLMNIdentity: plmnID, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: plmnID, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseEmergency,
		STMSI:                 &s1ap.STMSI{MMEC: code, MTMSI: mtmsi},
	}

	im, err := initialUE.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	conn := &captureConn{}
	HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(conn), initiatingValue(t, im))

	if conn.count() != 1 {
		t.Fatalf("expected one downlink (TAU Reject), got %d", conn.count())
	}

	rej, err := eps.ParseTrackingAreaUpdateReject(decodeDownlinkNAS(t, conn.sent[0]))
	if err != nil {
		t.Fatalf("parse TAU Reject: %v", err)
	}

	if rej.Cause != mme.EmmCauseUEIdentityUnderivable {
		t.Fatalf("TAU Reject cause = %d, want #%d", rej.Cause, mme.EmmCauseUEIdentityUnderivable)
	}
}

// TestInitialUEMessageResumeVerifiedBindsAndDispatches verifies the folded resume
// path end-to-end: a security-protected TRACKING AREA UPDATE presenting a valid
// S-TMSI and MAC binds the held context to the requesting connection (via the S-TMSI
// hint, mirroring the AMF), commits the uplink NAS COUNT, establishes secure exchange,
// and dispatches the message through the single HandleNAS entry (TS 24.301 §5.5.3.2).
func TestInitialUEMessageResumeVerifiedBindsAndDispatches(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code := m.MmeIdentity()
	m.ReallocateGUTI(ue, plmn, group, code)
	mtmsi := ue.TmsiForTest()

	tau, err := (&eps.TrackingAreaUpdateRequest{EPSUpdateType: 3}).Marshal() // periodic
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(tau, eps.SHTIntegrityProtected, nascommon.NASCount(0, 0),
		nascommon.DirectionUplink, ue.KnasIntForTest(), ue.KnasEncForTest(),
		nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	plmnID := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	initialUE := &s1ap.InitialUEMessage{
		ENBUES1APID:           1001,
		NASPDU:                s1ap.NASPDU(wire),
		TAI:                   s1ap.TAI{PLMNIdentity: plmnID, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: plmnID, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseEmergency,
		STMSI:                 &s1ap.STMSI{MMEC: code, MTMSI: mtmsi},
	}

	im, err := initialUE.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	conn := &captureConn{}
	HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(conn), initiatingValue(t, im))

	// The held context is bound to the resuming connection (the hint ran AttachUeConn).
	if !m.UeConnected(ue) {
		t.Fatal("UE not connected after a verified resume")
	}

	if got := ue.Conn().ENBUES1APID; got != 1001 {
		t.Fatalf("resumed connection eNB-UE-S1AP-ID = %d, want 1001", got)
	}

	// DecodeNASMessage (inside HandleNAS) committed the uplink NAS COUNT (0 -> 1) and
	// established secure exchange on the new connection — proof the message was decoded
	// and dispatched through the single entry, not bound blindly.
	if got := ue.ULCount(); got != 1 {
		t.Fatalf("uplink NAS COUNT = %d, want 1 (committed by the resume decode)", got)
	}

	if !ue.Conn().SecureExchangeEstablished() {
		t.Fatal("secure exchange not established on the resumed connection")
	}

	// The TAU was processed, not rejected.
	if conn.count() > 0 {
		if _, err := eps.ParseTrackingAreaUpdateReject(decodeDownlinkNAS(t, conn.sent[0])); err == nil {
			t.Fatal("verified resume was rejected; expected it to be accepted")
		}
	}
}

// decodeDownlinkNAS extracts the NAS PDU carried in a Downlink NAS Transport.
func decodeDownlinkNAS(t *testing.T, pdu []byte) []byte {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcDownlinkNASTransport {
		t.Fatalf("expected Downlink NAS Transport, got %T", msg)
	}

	dl, err := s1ap.ParseDownlinkNASTransport(im.Value)
	if err != nil {
		t.Fatalf("parse downlink: %v", err)
	}

	return []byte(dl.NASPDU)
}
