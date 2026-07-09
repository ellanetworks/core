// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/s1ap/aper"
)

// erabModIndicationWire builds an E-RAB MODIFICATION INDICATION initiatingMessage
// open-type payload, as an eNB would send it.
func erabModIndicationWire(t *testing.T, items []ERABToBeModifiedItemBearerModInd) []byte {
	t.Helper()

	var w aper.Writer

	w.WriteSequencePreamble(true, false, nil)

	err := encodeIEContainer(&w, []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: MMEUES1APID(1).encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: ENBUES1APID(2).encode},
		{id: idERABToBeModifiedListBearerModInd, crit: CriticalityReject, enc: func(w *aper.Writer) error {
			return encodeSingleContainerList(w, maxnoofERABs, idERABToBeModifiedItemBearerModInd, CriticalityReject, encoderList(items))
		}},
	})
	if err != nil {
		t.Fatalf("encode indication: %v", err)
	}

	return w.Bytes()
}

func TestERABModificationIndication_Decode(t *testing.T) {
	items := []ERABToBeModifiedItemBearerModInd{
		{ERABID: 5, TransportLayerAddress: TransportLayerAddress{10, 0, 0, 1}, DLGTPTEID: 0x11223344},
		{ERABID: 6, TransportLayerAddress: TransportLayerAddress{10, 0, 0, 2}, DLGTPTEID: 0x55667788},
	}

	msg, err := ParseERABModificationIndication(erabModIndicationWire(t, items))
	if err != nil {
		t.Fatalf("ParseERABModificationIndication: %v", err)
	}

	if msg.MMEUES1APID != 1 || msg.ENBUES1APID != 2 {
		t.Fatalf("UE IDs = (%d,%d), want (1,2)", msg.MMEUES1APID, msg.ENBUES1APID)
	}

	if len(msg.ToBeModified) != 2 {
		t.Fatalf("ToBeModified count = %d, want 2", len(msg.ToBeModified))
	}

	for i, want := range items {
		got := msg.ToBeModified[i]
		if got.ERABID != want.ERABID || got.DLGTPTEID != want.DLGTPTEID ||
			!bytes.Equal(got.TransportLayerAddress, want.TransportLayerAddress) {
			t.Fatalf("item %d = %+v, want %+v", i, got, want)
		}
	}
}

func TestERABModificationIndication_MissingMandatoryIE(t *testing.T) {
	var w aper.Writer

	w.WriteSequencePreamble(true, false, nil)

	// Only the UE IDs, no E-RABToBeModified list (mandatory).
	if err := encodeIEContainer(&w, []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: MMEUES1APID(1).encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: ENBUES1APID(2).encode},
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	if _, err := ParseERABModificationIndication(w.Bytes()); err == nil {
		t.Fatal("expected error for missing E-RABToBeModified list, got nil")
	}
}

func TestERABModificationConfirm_Marshal(t *testing.T) {
	wire, err := (&ERABModificationConfirm{MMEUES1APID: 1, ENBUES1APID: 2, ModifiedERABs: []ERABID{5, 6}}).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	pdu, err := Unmarshal(wire)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcERABModificationIndication {
		t.Fatalf("expected SuccessfulOutcome/ProcERABModificationIndication, got %T proc %d", pdu, pdu.procedureCode())
	}

	// The confirm body must carry the E-RABModifyListBearerModConf IE.
	r := aper.NewReader(so.Value)
	if _, _, err := r.ReadSequencePreamble(true, 0); err != nil {
		t.Fatalf("body preamble: %v", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		t.Fatalf("decode container: %v", err)
	}

	var seenList bool

	for _, f := range fields {
		if f.id == idERABModifyListBearerModConf {
			seenList = true
		}
	}

	if !seenList {
		t.Fatal("E-RABModifyListBearerModConf IE missing from confirm")
	}
}
