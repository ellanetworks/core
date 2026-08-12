// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

func erabModIndicationWire(t *testing.T, items []ERABToBeModifiedItemBearerModInd) []byte {
	t.Helper()

	w := per.NewWriter()

	w.WriteBit(false)

	err := encodeIEContainer(w, per.Aligned, []ieField{
		{id: IDMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		{id: IDENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(2)},
		{id: IDERABToBeModifiedListBearerModInd, crit: CriticalityReject, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeModifiedItemBearerModInd, CriticalityReject, items)
		})},
	})
	if err != nil {
		t.Fatalf("encode indication: %v", err)
	}

	return perBytes(w)
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
	w := per.NewWriter()

	w.WriteBit(false)

	// Only the UE IDs, no E-RABToBeModified list (mandatory).
	if err := encodeIEContainer(w, per.Aligned, []ieField{
		{id: IDMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		{id: IDENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(2)},
	}); err != nil {
		t.Fatalf("encode: %v", err)
	}

	if _, err := ParseERABModificationIndication(perBytes(w)); err == nil {
		t.Fatal("expected error for missing E-RABToBeModified list, got nil")
	}
}

func TestERABModificationConfirm_Marshal(t *testing.T) {
	wire, err := (&ERABModificationConfirm{MMEUES1APID: Ptr(MMEUES1APID(1)), ENBUES1APID: Ptr(ENBUES1APID(2)), ModifiedERABs: []ERABID{5, 6}}).Marshal()
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
	r := per.NewReader(so.Value)
	if _, err := r.ReadBit(); err != nil {
		t.Fatalf("body preamble: %v", err)
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		t.Fatalf("decode container: %v", err)
	}

	var seenList bool

	for _, f := range fields {
		if f.id == IDERABModifyListBearerModConf {
			seenList = true
		}
	}

	if !seenList {
		t.Fatal("E-RABModifyListBearerModConf IE missing from confirm")
	}
}
