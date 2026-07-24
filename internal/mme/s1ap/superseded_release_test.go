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

// resumeOntoNewConnection registers a secured, ECM-CONNECTED UE on oldConn
// (eNB-UE-S1AP-ID 7) and then has it re-establish on a new S1 connection via a
// verified TRACKING AREA UPDATE, mirroring the flow from issue #1482. It returns the
// old connection's MME/eNB S1AP IDs.
func resumeOntoNewConnection(t *testing.T, m *mme.MME, ue *mme.UeContext) (oldMMEID s1ap.MMEUES1APID, oldENBID s1ap.ENBUES1APID) {
	t.Helper()

	oldMMEID = ue.Conn().MMEUES1APID
	oldENBID = ue.Conn().ENBUES1APID

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code := m.MmeIdentity()
	if _, err := m.ReallocateGUTI(t.Context(), ue, plmn, group, code); err != nil {
		t.Fatal(err)
	}

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

	im, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           1001,
		NASPDU:                s1ap.NASPDU(wire),
		TAI:                   s1ap.TAI{PLMNIdentity: plmnID, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: plmnID, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseEmergency,
		STMSI:                 &s1ap.STMSI{MMEC: code, MTMSI: ue.TmsiForTest()},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	newConn := &captureConn{}
	HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(newConn), initiatingValue(t, im))

	if ue.Conn().ENBUES1APID != 1001 {
		t.Fatalf("resume did not bind the new connection (eNB-UE-S1AP-ID = %d)", ue.Conn().ENBUES1APID)
	}

	return oldMMEID, oldENBID
}

// isReleaseCommandFor reports whether pdu is a UE CONTEXT RELEASE COMMAND naming the
// given S1AP ID pair.
func isReleaseCommandFor(t *testing.T, pdu []byte, mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID) bool {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcUEContextRelease {
		return false
	}

	cmd, err := s1ap.ParseUEContextReleaseCommand(im.Value)
	if err != nil {
		t.Fatalf("parse UE Context Release Command: %v", err)
	}

	return cmd.UES1APIDs.MMEUES1APID == mmeID && cmd.UES1APIDs.ENBUES1APID == enbID
}

func isErrorIndication(t *testing.T, pdu []byte) bool {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)

	return ok && im.ProcedureCode == s1ap.ProcErrorIndication
}

// TestSupersededConnectionIsReleasedTowardENB verifies the fix for the 4G/S1AP side of
// issue #1482: when a UE re-establishes on a new S1 connection, the MME must release
// the old (superseded) connection toward the eNB with a UE CONTEXT RELEASE COMMAND
// (TS 23.401 §4.11, TS 36.413 §8.3.3.1) — not drop it silently, which leaves the eNB's
// stale context dangling and its later release request unanswerable.
func TestSupersededConnectionIsReleasedTowardENB(t *testing.T) {
	m := newTestMME(t)
	ue, oldConn := securedUE(t, m) // ECM-CONNECTED on oldConn, eNB-UE-S1AP-ID 7

	oldMMEID, oldENBID := resumeOntoNewConnection(t, m, ue)

	var released bool

	for _, pdu := range oldConn.sent {
		if isReleaseCommandFor(t, pdu, oldMMEID, oldENBID) {
			released = true
		}
	}

	if !released {
		t.Fatalf("issue #1482: MME did not send a UE CONTEXT RELEASE COMMAND for the superseded "+
			"context (MME-UE-S1AP-ID %d); it dropped the old S1 connection silently", oldMMEID)
	}
}

// TestSupersededConnectionReleaseRequestGetsCommand verifies that an eNB release
// request for the superseded context — one that crossed the MME's own release command —
// is answered with a UE CONTEXT RELEASE COMMAND (TS 36.413 §8.3.2.2), never an Error
// Indication with unknown-mme-ue-s1ap-id.
func TestSupersededConnectionReleaseRequestGetsCommand(t *testing.T) {
	m := newTestMME(t)
	ue, oldConn := securedUE(t, m)

	oldMMEID, oldENBID := resumeOntoNewConnection(t, m, ue)

	relReq, err := (&s1ap.UEContextReleaseRequest{
		MMEUES1APID: oldMMEID,
		ENBUES1APID: oldENBID,
		Cause:       s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioConnectionWithUELost},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	before := oldConn.count()
	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(oldConn), initiatingValue(t, relReq))

	sent := oldConn.sent[before:]
	if len(sent) != 1 {
		t.Fatalf("expected exactly one S1AP response to the release request, got %d", len(sent))
	}

	if isErrorIndication(t, sent[0]) {
		t.Fatalf("issue #1482: MME answered the eNB's release request for the superseded context "+
			"(MME-UE-S1AP-ID %d) with ERROR INDICATION; TS 36.413 §8.3.2.2 requires a UE CONTEXT "+
			"RELEASE COMMAND", oldMMEID)
	}

	if !isReleaseCommandFor(t, sent[0], oldMMEID, oldENBID) {
		t.Fatalf("expected a UE CONTEXT RELEASE COMMAND for the superseded context")
	}
}
