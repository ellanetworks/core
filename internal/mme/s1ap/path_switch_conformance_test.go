// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// Conformance tests for the X2 handover Path Switch procedure, derived clause by
// clause from TS 36.413 §8.4.4 and §9.1.5.8-10, TS 23.401 §5.5.1.1.2 and
// TS 33.401 §7.2.8.4.2.

const (
	ieIDTAI                    = 67
	ieIDEUTRANCGI              = 100
	ieIDUESecurityCapabilities = 107
)

var errAnchorRefused = errors.New("anchor refused the downlink switch")

func dropIEs(t *testing.T, value []byte, ids ...uint16) []byte {
	t.Helper()

	drop := make(map[uint16]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}

	if len(value) < 3 {
		t.Fatalf("message body too short: %d bytes", len(value))
	}

	preamble, count, pos := value[0], binary.BigEndian.Uint16(value[1:3]), 3

	kept := make([]byte, 0, len(value))
	keptCount := uint16(0)

	for i := uint16(0); i < count; i++ {
		start := pos
		if pos+4 > len(value) {
			t.Fatalf("truncated IE header at offset %d", pos)
		}

		id := binary.BigEndian.Uint16(value[pos : pos+2])
		pos += 3 // ID (2) + criticality (1)

		n := int(value[pos])
		if n < 0x80 {
			pos++
		} else if n < 0xC0 {
			if pos+2 > len(value) {
				t.Fatalf("truncated length determinant at offset %d", pos)
			}

			n = (n&0x3F)<<8 | int(value[pos+1])
			pos += 2
		} else {
			t.Fatalf("fragmented IE length at offset %d is not supported", pos)
		}

		if pos+n > len(value) {
			t.Fatalf("IE 0x%04x claims %d bytes, only %d remain", id, n, len(value)-pos)
		}

		pos += n

		if drop[id] {
			continue
		}

		kept = append(kept, value[start:pos]...)
		keptCount++
	}

	out := make([]byte, 3, 3+len(kept))
	out[0] = preamble
	binary.BigEndian.PutUint16(out[1:3], keptCount)

	return append(out, kept...)
}

func pathSwitchUEWithSecondPDN(t *testing.T, m *mme.MME) (*mme.UeContext, *mme.PdnConnection, *mme.PdnConnection) {
	t.Helper()

	ue := pathSwitchUE(t, m)

	first := testPDN(ue)
	first.SessionRef = "session-ebi-5"

	second := ue.EnsurePDN(6)
	second.Apn = "internet2"
	second.SessionRef = "session-ebi-6"

	return ue, first, second
}

func switchedDLItemFor(ebi uint8, teid uint32) s1ap.ERABToBeSwitchedDLItem {
	return s1ap.ERABToBeSwitchedDLItem{
		ERABID:                s1ap.ERABID(ebi),
		TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
		GTPTEID:               s1ap.GTPTEID(teid),
	}
}

func sessionManager(t *testing.T, m *mme.MME) *fakeSessionManager {
	t.Helper()

	fsm, ok := m.Session.(*fakeSessionManager)
	if !ok {
		t.Fatalf("session manager is %T, want *fakeSessionManager", m.Session)
	}

	return fsm
}

func releasedRef(fsm *fakeSessionManager, ref string) bool {
	for _, r := range fsm.releasedRefs {
		if r == ref {
			return true
		}
	}

	return false
}

func TestPathSwitchImplicitlyReleasesOmittedERABs(t *testing.T) {
	m := newTestMME(t)
	ue, _, second := pathSwitchUEWithSecondPDN(t, m)
	fsm := sessionManager(t, m)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL = []s1ap.ERABToBeSwitchedDLItem{switchedDLItemFor(mme.DefaultERABID, 0x99)}

	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(&captureConn{}), pathSwitchValue(t, req))

	if !releasedRef(fsm, second.SessionRef) {
		t.Errorf("omitted E-RAB 6 was not released at the anchor; released refs = %v", fsm.releasedRefs)
	}

	for _, p := range m.SnapshotPDNs(ue) {
		if p.Ebi == 6 {
			t.Errorf("omitted E-RAB 6 is still in the UE context with eNB F-TEID %+v", p.EnbFTEID)
		}
	}
}

func TestPathSwitchOmittedDefaultBearerReleasesPDNConnection(t *testing.T) {
	m := newTestMME(t)
	ue, first, _ := pathSwitchUEWithSecondPDN(t, m)
	fsm := sessionManager(t, m)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL = []s1ap.ERABToBeSwitchedDLItem{switchedDLItemFor(6, 0xaa)}

	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(&captureConn{}), pathSwitchValue(t, req))

	if !releasedRef(fsm, first.SessionRef) {
		t.Errorf("PDN connection whose default bearer was not accepted was not released; released refs = %v", fsm.releasedRefs)
	}
}

func TestPathSwitchAllUPSwitchesFailSendsFailure(t *testing.T) {
	m := newTestMME(t)
	ue, _, _ := pathSwitchUEWithSecondPDN(t, m)
	fsm := sessionManager(t, m)

	fsm.failModify(mme.DefaultERABID, errAnchorRefused)
	fsm.failModify(6, errAnchorRefused)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL = []s1ap.ERABToBeSwitchedDLItem{
		switchedDLItemFor(mme.DefaultERABID, 0x99),
		switchedDLItemFor(6, 0xaa),
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

	if target.count() != 1 {
		t.Fatalf("sent %d messages, want exactly one Path Switch Request Failure", target.count())
	}

	fail := parsePathSwitchFailure(t, target.sent[0])
	if fail.Cause == nil {
		t.Fatal("Path Switch Request Failure carries no Cause")
	}
}

func TestPathSwitchPartialUPFailureReportsReleasedERAB(t *testing.T) {
	m := newTestMME(t)
	ue, _, _ := pathSwitchUEWithSecondPDN(t, m)
	fsm := sessionManager(t, m)

	fsm.failModify(6, errAnchorRefused)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL = []s1ap.ERABToBeSwitchedDLItem{
		switchedDLItemFor(mme.DefaultERABID, 0x99),
		switchedDLItemFor(6, 0xaa),
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

	if target.count() != 1 {
		t.Fatalf("sent %d messages, want exactly one Path Switch Request Acknowledge", target.count())
	}

	ack := parsePathSwitchAck(t, target.sent[0])

	if len(ack.ERABToBeReleased) != 1 || ack.ERABToBeReleased[0].ERABID != 6 {
		t.Fatalf("E-RAB To Be Released List = %+v, want exactly E-RAB 6", ack.ERABToBeReleased)
	}

	// TS 36.413 §9.1.5.9: an E-RAB ID shall be present only once across the
	// switched-uplink and to-be-released lists.
	seen := map[s1ap.ERABID]bool{}
	for _, item := range ack.ERABToBeReleased {
		if seen[item.ERABID] {
			t.Errorf("E-RAB ID %d appears more than once in the To Be Released List", item.ERABID)
		}

		seen[item.ERABID] = true
	}
}

func TestPathSwitchFailedERABReleasesCoreNetworkResources(t *testing.T) {
	m := newTestMME(t)
	ue, _, second := pathSwitchUEWithSecondPDN(t, m)
	fsm := sessionManager(t, m)

	fsm.failModify(6, errAnchorRefused)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL = []s1ap.ERABToBeSwitchedDLItem{
		switchedDLItemFor(mme.DefaultERABID, 0x99),
		switchedDLItemFor(6, 0xaa),
	}

	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(&captureConn{}), pathSwitchValue(t, req))

	if !releasedRef(fsm, second.SessionRef) {
		t.Errorf("core-network resources of the failed E-RAB 6 were not released; released refs = %v", fsm.releasedRefs)
	}
}

func TestPathSwitchTotalFailureDetachesUE(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)
	testPDN(ue).SessionRef = "session-ebi-5"

	fsm := sessionManager(t, m)
	fsm.failModify(mme.DefaultERABID, errAnchorRefused)

	// Captured before the handler runs: an explicit detach tears the association down.
	mmeID := ue.Conn().MMEUES1APID

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if !releasedRef(fsm, "session-ebi-5") {
		t.Errorf("UE was not detached after a total path-switch failure; released refs = %v", fsm.releasedRefs)
	}

	if _, stillKnown := m.LookupUe(mmeID); stillKnown {
		t.Error("UE is still registered after a total path-switch failure; no explicit detach was performed")
	}
}

func TestPathSwitchAckCarriesUEAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, _, _ := pathSwitchUEWithSecondPDN(t, m)

	ue.Ambr = &models.Ambr{
		Uplink:   models.MustParseBitRate("100 Mbps"),
		Downlink: models.MustParseBitRate("200 Mbps"),
	}

	fsm := sessionManager(t, m)
	fsm.failModify(6, errAnchorRefused)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL = []s1ap.ERABToBeSwitchedDLItem{
		switchedDLItemFor(mme.DefaultERABID, 0x99),
		switchedDLItemFor(6, 0xaa),
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

	if target.count() != 1 {
		t.Fatalf("sent %d messages, want exactly one Path Switch Request Acknowledge", target.count())
	}

	ack := parsePathSwitchAck(t, target.sent[0])
	if ack.UEAggregateMaximumBitRate == nil {
		t.Fatal("Path Switch Request Acknowledge carries no UE Aggregate Maximum Bit Rate")
	}
}

func TestPathSwitchNCCWrapsToZero(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)
	ue.SetNCCForTest(7)

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if target.count() != 1 {
		t.Fatalf("sent %d messages, want exactly one Path Switch Request Acknowledge", target.count())
	}

	ack := parsePathSwitchAck(t, target.sent[0])
	if ack.SecurityContext.NextHopChainingCount != 0 {
		t.Errorf("NCC after 7 = %d, want 0", ack.SecurityContext.NextHopChainingCount)
	}

	if ue.NCCForTest() != 0 {
		t.Errorf("stored NCC after 7 = %d, want 0", ue.NCCForTest())
	}
}

func TestPathSwitchAckCarriesFreshlyDerivedNH(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	beforeNH, beforeNCC := ue.NHForTest(), ue.NCCForTest()

	wantNH, err := ue.DeriveNextNHForTest()
	if err != nil {
		t.Fatal(err)
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, samplePathSwitchRequest(ue)))

	ack := parsePathSwitchAck(t, target.sent[0])

	if ack.SecurityContext.NextHopParameter == s1ap.SecurityKey(beforeNH) {
		t.Error("acknowledge replayed the current NH instead of a freshly derived one")
	}

	if ack.SecurityContext.NextHopParameter != s1ap.SecurityKey(wantNH) {
		t.Error("acknowledge NH is not KDF(KASME, previous NH)")
	}

	if ack.SecurityContext.NextHopChainingCount != (beforeNCC+1)&0x07 {
		t.Errorf("acknowledge NCC = %d, want %d", ack.SecurityContext.NextHopChainingCount, (beforeNCC+1)&0x07)
	}
}

func TestPathSwitchFailureCarriesMandatoryIEs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		prepare func(t *testing.T, m *mme.MME) *s1ap.PathSwitchRequest
	}{
		{
			name: "duplicate E-RAB ID",
			prepare: func(t *testing.T, m *mme.MME) *s1ap.PathSwitchRequest {
				ue := pathSwitchUE(t, m)
				req := samplePathSwitchRequest(ue)
				req.ERABToBeSwitchedDL = []s1ap.ERABToBeSwitchedDLItem{switchedDLItem(), switchedDLItem()}

				return req
			},
		},
		{
			name: "unknown source MME UE S1AP ID",
			prepare: func(t *testing.T, m *mme.MME) *s1ap.PathSwitchRequest {
				ue := pathSwitchUE(t, m)
				req := samplePathSwitchRequest(ue)
				req.SourceMMEUES1APID = 0x7ffffff0

				return req
			},
		},
		{
			name: "no E-RAB switched",
			prepare: func(t *testing.T, m *mme.MME) *s1ap.PathSwitchRequest {
				ue := pathSwitchUE(t, m)
				sessionManager(t, m).failModify(mme.DefaultERABID, errAnchorRefused)

				return samplePathSwitchRequest(ue)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMME(t)
			req := tt.prepare(t, m)

			target := &captureConn{}
			handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

			if target.count() != 1 {
				t.Fatalf("sent %d messages, want exactly one Path Switch Request Failure", target.count())
			}

			fail := parsePathSwitchFailure(t, target.sent[0])

			if fail.MMEUES1APID == nil {
				t.Error("failure carries no MME UE S1AP ID")
			}

			if fail.ENBUES1APID == nil {
				t.Error("failure carries no eNB UE S1AP ID")
			}

			if fail.Cause == nil {
				t.Error("failure carries no Cause")
			}
		})
	}
}

func TestPathSwitchToleratesAbsentIgnoreCriticalityIEs(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	value := dropIEs(t, pathSwitchValue(t, samplePathSwitchRequest(ue)),
		ieIDEUTRANCGI, ieIDTAI, ieIDUESecurityCapabilities)

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), value)

	if target.count() != 1 {
		t.Fatalf("sent %d messages, want exactly one Path Switch Request Acknowledge", target.count())
	}

	if _, err := s1ap.Unmarshal(target.sent[0]); err != nil {
		t.Fatalf("unmarshal reply: %v", err)
	}

	ack := parsePathSwitchAck(t, target.sent[0])
	if ack.SecurityContext.NextHopChainingCount != 2 {
		t.Errorf("NCC = %d, want 2: the path switch did not complete", ack.SecurityContext.NextHopChainingCount)
	}
}

func TestPathSwitchDuringNASSecurityModeStillAcknowledges(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	oldNH, oldNCC := ue.NHForTest(), ue.NCCForTest()

	wantNH, err := ue.DeriveNextNHForTest()
	if err != nil {
		t.Fatal(err)
	}

	if !m.TryClaimKeyChain(ue) {
		t.Fatal("could not claim the key chain for the NAS security mode procedure")
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if target.count() != 1 {
		t.Fatalf("sent %d messages, want exactly one Path Switch Request Acknowledge", target.count())
	}

	ack := parsePathSwitchAck(t, target.sent[0])

	if ack.SecurityContext.NextHopParameter != s1ap.SecurityKey(wantNH) {
		t.Errorf("acknowledge NH is not derived from the old KASME chain (previous NH %x, NCC %d)", oldNH[:4], oldNCC)
	}
}
