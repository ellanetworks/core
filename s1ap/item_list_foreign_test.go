// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"slices"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

const idForeignListItem ProtocolIEID = 9999

func erabSetupResponseWithList(t *testing.T, fields []apimpl.FieldValue[ProtocolIEID]) []byte {
	t.Helper()

	w := per.NewWriter()

	err := per.EncodeLength(w, per.Aligned, 1, maxnoofERABs, true, int64(len(fields)), func(int64) error {
		for _, f := range fields {
			if err := encodeContainerField(w, per.Aligned, f); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	b, err := (&ERABSetupResponse{
		MMEUES1APID: Ptr(MMEUES1APID(1)),
		ENBUES1APID: Ptr(ENBUES1APID(2)),
		ERABSetup:   ERABSetupListBearerSURes{{ERABID: 1, TransportLayerAddress: goldTLA()}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	raw, err := ParseRawPDU(b)
	if err != nil {
		t.Fatal(err)
	}

	i, err := raw.Index(IDERABSetupListBearerSURes)
	if err != nil {
		t.Fatal(err)
	}

	if err := raw.SetValue(i, w.Bytes()); err != nil {
		t.Fatal(err)
	}

	out, err := raw.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func setupItemField(id ERABID) apimpl.FieldValue[ProtocolIEID] {
	return apimpl.FieldValue[ProtocolIEID]{
		ID:          IDERABSetupItemBearerSURes,
		Criticality: CriticalityIgnore,
		Value:       &ERABSetupItemBearerSURes{ERABID: id, TransportLayerAddress: goldTLA(), GTPTEID: GTPTEID(id)},
	}
}

func foreignItemField(crit Criticality) apimpl.FieldValue[ProtocolIEID] {
	return apimpl.FieldValue[ProtocolIEID]{ID: idForeignListItem, Criticality: crit, Value: apimpl.RawValue{0x01, 0x02}}
}

func TestItemListWithForeignIEIsAnAbstractSyntaxError(t *testing.T) {
	for _, c := range []struct {
		name   string
		fields []apimpl.FieldValue[ProtocolIEID]
		erabs  []ERABID
		crit   Criticality
		reject bool
	}{
		{"ignore before item", []apimpl.FieldValue[ProtocolIEID]{foreignItemField(CriticalityIgnore), setupItemField(5)}, []ERABID{5}, CriticalityIgnore, false},
		{"notify between items", []apimpl.FieldValue[ProtocolIEID]{setupItemField(5), foreignItemField(CriticalityNotify), setupItemField(6)}, []ERABID{5, 6}, CriticalityNotify, false},
		{"reject after item", []apimpl.FieldValue[ProtocolIEID]{setupItemField(5), foreignItemField(CriticalityReject)}, []ERABID{5}, CriticalityReject, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := erabSetupResponseWithList(t, c.fields)

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			m, err := DecodeERABSetupResponse(pdu.value())
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			var got []ERABID
			for _, it := range m.ERABSetup {
				got = append(got, it.ERABID)
			}

			if !slices.Equal(got, c.erabs) {
				t.Errorf("E-RABs = %v, want %v", got, c.erabs)
			}

			want := DiagnosticIE{Path: []ProtocolIEID{IDERABSetupListBearerSURes}, ID: idForeignListItem, Criticality: c.crit, Kind: ap.KindNotComprehended}

			if ies := m.Diagnostics().IEs; len(ies) != 1 || ies[0].String() != want.String() {
				t.Fatalf("diagnostics = %v, want %v", ies, want)
			}

			re, err := m.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(re, b) {
				t.Fatalf("re-encode = % x, want % x", re, b)
			}

			if _, err := ParseERABSetupResponse(pdu.value()); (err != nil) != c.reject {
				t.Fatalf("parse = %v, want reject %v", err, c.reject)
			}
		})
	}
}

func TestItemListOfOnlyForeignIEsIsNotComprehended(t *testing.T) {
	b := erabSetupResponseWithList(t, []apimpl.FieldValue[ProtocolIEID]{foreignItemField(CriticalityIgnore)})

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	m, err := DecodeERABSetupResponse(pdu.value())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(m.ERABSetup) != 1 || len(m.ERABSetup[0].ForeignIEs()) != 1 {
		t.Errorf("ERABSetup = %+v, want the foreign IE kept in the list", m.ERABSetup)
	}

	var kinds []string
	for _, d := range m.Diagnostics().IEs {
		kinds = append(kinds, d.String())
	}

	want := []string{
		DiagnosticIE{Path: []ProtocolIEID{IDERABSetupListBearerSURes}, ID: idForeignListItem, Criticality: CriticalityIgnore}.String(),
		DiagnosticIE{ID: IDERABSetupListBearerSURes, Criticality: CriticalityIgnore}.String(),
	}

	if !slices.Equal(kinds, want) {
		t.Errorf("diagnostics = %v, want %v", kinds, want)
	}

	re, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(re, b) {
		t.Fatalf("re-encode = % x, want % x", re, b)
	}

	parsed, err := ParseERABSetupResponse(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	if parsed.ERABSetup != nil {
		t.Errorf("parsed ERABSetup = %+v, want the not comprehended list treated as not received", parsed.ERABSetup)
	}
}
