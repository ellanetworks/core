// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// erabModValue marshals an E-RAB MODIFICATION INDICATION and returns the
// initiatingMessage open-type payload the handler consumes.
func erabModValue(t *testing.T, req *s1ap.ERABModificationIndication) []byte {
	t.Helper()

	b, err := req.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return pdu.(*s1ap.InitiatingMessage).Value
}

func modifiedItem(addr [4]byte, teid uint32) s1ap.ERABToBeModifiedItemBearerModInd {
	return s1ap.ERABToBeModifiedItemBearerModInd{
		ERABID:                s1ap.ERABID(mme.DefaultERABID),
		TransportLayerAddress: s1ap.TransportLayerAddress(addr[:]),
		DLGTPTEID:             s1ap.GTPTEID(teid),
	}
}

func TestERABModificationIndication_RelocatesAndConfirms(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue) // default bearer, EBI 5

	req := &s1ap.ERABModificationIndication{
		MMEUES1APID:  ue.Conn().MMEUES1APID,
		ENBUES1APID:  ue.Conn().ENBUES1APID,
		ToBeModified: []s1ap.ERABToBeModifiedItemBearerModInd{modifiedItem([4]byte{10, 5, 0, 2}, 0x1234)},
	}

	handleERABModificationIndication(m, context.Background(), mme.NewRadioForTest(cc), erabModValue(t, req))

	// Downlink relocated to the reported eNB S1-U endpoint.
	wantFTEID := models.FTEID{TEID: 0x1234, Addr: netip.AddrFrom4([4]byte{10, 5, 0, 2})}
	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != wantFTEID {
		t.Fatalf("ModifyEPSSession eNB F-TEID = %+v, want %+v", fsm.modifiedENB, wantFTEID)
	}

	if testPDN(ue).EnbFTEID != wantFTEID {
		t.Fatalf("stored eNB F-TEID = %+v, want %+v", testPDN(ue).EnbFTEID, wantFTEID)
	}

	// A single E-RAB MODIFICATION CONFIRM was sent.
	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 E-RAB Modification Confirm, got %d S1AP messages", len(cc.sent))
	}

	pdu, err := s1ap.Unmarshal(cc.sent[0])
	if err != nil {
		t.Fatalf("unmarshal confirm: %v", err)
	}

	so, ok := pdu.(*s1ap.SuccessfulOutcome)
	if !ok || so.ProcedureCode != s1ap.ProcERABModificationIndication {
		t.Fatalf("expected E-RAB Modification Confirm, got %T", pdu)
	}
}

// TestERABModificationIndication_OmittedERABReleases covers TS 36.413 §8.2.4.4:
// an indication that omits an established E-RAB triggers a UE Context Release, not
// a modification.
func TestERABModificationIndication_OmittedERABReleases(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue)     // default bearer, EBI 5
	ue.EnsurePDN(6) // a second bearer the indication will omit

	req := &s1ap.ERABModificationIndication{
		MMEUES1APID:  ue.Conn().MMEUES1APID,
		ENBUES1APID:  ue.Conn().ENBUES1APID,
		ToBeModified: []s1ap.ERABToBeModifiedItemBearerModInd{modifiedItem([4]byte{10, 5, 0, 2}, 0x1234)},
	}

	handleERABModificationIndication(m, context.Background(), mme.NewRadioForTest(cc), erabModValue(t, req))

	// No downlink was modified — the abnormal condition took over.
	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("expected no E-RAB modification, got F-TEID %+v", fsm.modifiedENB)
	}

	// The sole message is a UE Context Release Command, not a Confirm.
	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 UE Context Release Command, got %d S1AP messages", len(cc.sent))
	}

	pdu, err := s1ap.Unmarshal(cc.sent[0])
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok {
		t.Fatalf("expected UE Context Release Command (InitiatingMessage), got %T", pdu)
	}

	if im.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command, got proc %d", im.ProcedureCode)
	}
}

// TestERABModificationIndication_DuplicateERABReleases covers TS 36.413 §8.2.4.4:
// a repeated E-RAB ID triggers a UE Context Release.
func TestERABModificationIndication_DuplicateERABReleases(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue)

	req := &s1ap.ERABModificationIndication{
		MMEUES1APID: ue.Conn().MMEUES1APID,
		ENBUES1APID: ue.Conn().ENBUES1APID,
		ToBeModified: []s1ap.ERABToBeModifiedItemBearerModInd{
			modifiedItem([4]byte{10, 5, 0, 2}, 0x1234),
			modifiedItem([4]byte{10, 5, 0, 3}, 0x5678),
		},
	}

	handleERABModificationIndication(m, context.Background(), mme.NewRadioForTest(cc), erabModValue(t, req))

	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("expected no E-RAB modification on duplicate, got F-TEID %+v", fsm.modifiedENB)
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 UE Context Release Command, got %d", len(cc.sent))
	}

	pdu, err := s1ap.Unmarshal(cc.sent[0])
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if im, ok := pdu.(*s1ap.InitiatingMessage); !ok || im.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command, got %T", pdu)
	}
}
