// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

// The AMF keys radios on the rendered RAN node id and reports it over the API,
// so the in-house conversion has to produce the string the free5gc-based one
// produced. These compare the two directly; they go once free5gc leaves go.mod.

func TestRANNodeIDToModelsMatchesFree5GC(t *testing.T) {
	tests := []struct {
		name  string
		kind  ngap.RANNodeIDKind
		value uint32
		bits  int
	}{
		{"gNB 22 bits", ngap.RANNodeIDGNB, 0x3fabcd, 22},
		{"gNB 24 bits", ngap.RANNodeIDGNB, 0x000102, 24},
		{"gNB 28 bits", ngap.RANNodeIDGNB, 0xabcdef1, 28},
		{"gNB 32 bits", ngap.RANNodeIDGNB, 0xdeadbeef, 32},
		{"macro ng-eNB", ngap.RANNodeIDMacroNgENB, 0xabcde, 20},
		{"short macro ng-eNB", ngap.RANNodeIDShortMacroNgENB, 0x3abcd, 18},
		{"long macro ng-eNB", ngap.RANNodeIDLongMacroNgENB, 0x1abcde, 21},
		{"N3IWF", ngap.RANNodeIDN3IWF, 0xbeef, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.RANNodeIDToModels(ngap.GlobalRANNodeID{
				Kind:         tt.kind,
				PLMNIdentity: ngap.PLMNIdentity{0x02, 0xf8, 0x39},
				Value:        tt.value,
				Bits:         tt.bits,
			})

			want := ngapConvert.RanIdToModels(free5gcRanID(t, tt.kind, tt.value, tt.bits))

			if got.N3IwfID != want.N3IwfId || got.NgeNbID != want.NgeNbId {
				t.Errorf("got %+v, want %+v", got, want)
			}

			if (got.GNbID == nil) != (want.GNbId == nil) {
				t.Fatalf("gNB id presence: got %+v, want %+v", got.GNbID, want.GNbId)
			}

			if got.GNbID != nil &&
				(got.GNbID.GNBValue != want.GNbId.GNBValue || got.GNbID.BitLength != want.GNbId.BitLength) {
				t.Errorf("gNB id = %+v, want %+v", *got.GNbID, *want.GNbId)
			}
		})
	}
}

// free5gcRanID builds the equivalent free5gc value for the same identifier.
func free5gcRanID(t *testing.T, kind ngap.RANNodeIDKind, value uint32, bits int) ngapType.GlobalRANNodeID {
	t.Helper()

	bs := aper.BitString{Bytes: leftAligned(value, bits), BitLength: uint64(bits)}
	plmn := ngapType.PLMNIdentity{Value: []byte{0x02, 0xf8, 0x39}}

	var out ngapType.GlobalRANNodeID

	switch kind {
	case ngap.RANNodeIDGNB:
		out.Present = ngapType.GlobalRANNodeIDPresentGlobalGNBID
		out.GlobalGNBID = &ngapType.GlobalGNBID{PLMNIdentity: plmn}
		out.GlobalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
		out.GlobalGNBID.GNBID.GNBID = &bs
	case ngap.RANNodeIDMacroNgENB, ngap.RANNodeIDShortMacroNgENB, ngap.RANNodeIDLongMacroNgENB:
		out.Present = ngapType.GlobalRANNodeIDPresentGlobalNgENBID
		out.GlobalNgENBID = &ngapType.GlobalNgENBID{PLMNIdentity: plmn}

		switch kind {
		case ngap.RANNodeIDMacroNgENB:
			out.GlobalNgENBID.NgENBID.Present = ngapType.NgENBIDPresentMacroNgENBID
			out.GlobalNgENBID.NgENBID.MacroNgENBID = &bs
		case ngap.RANNodeIDShortMacroNgENB:
			out.GlobalNgENBID.NgENBID.Present = ngapType.NgENBIDPresentShortMacroNgENBID
			out.GlobalNgENBID.NgENBID.ShortMacroNgENBID = &bs
		case ngap.RANNodeIDLongMacroNgENB:
			out.GlobalNgENBID.NgENBID.Present = ngapType.NgENBIDPresentLongMacroNgENBID
			out.GlobalNgENBID.NgENBID.LongMacroNgENBID = &bs
		}
	case ngap.RANNodeIDN3IWF:
		out.Present = ngapType.GlobalRANNodeIDPresentGlobalN3IWFID
		out.GlobalN3IWFID = &ngapType.GlobalN3IWFID{PLMNIdentity: plmn}
		out.GlobalN3IWFID.N3IWFID.Present = ngapType.N3IWFIDPresentN3IWFID
		out.GlobalN3IWFID.N3IWFID.N3IWFID = &bs
	}

	return out
}

func leftAligned(v uint32, nbits int) []byte {
	b := make([]byte, (nbits+7)/8)
	for i := range nbits {
		if v&(1<<uint(nbits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return b
}

// GUAMIToNGAP splits the AMF id the same way free5gc's AmfIdToNgap does, which
// is what decides the GUAMI a gNB sees in NG Setup Response.
func TestGUAMIToNGAPMatchesFree5GC(t *testing.T) {
	for _, amfID := range []string{"000000", "cafe00", "020040", "ffffff", "123456"} {
		t.Run(amfID, func(t *testing.T) {
			got, err := util.GUAMIToNGAP(models.Guami{
				PlmnID: &models.PlmnID{Mcc: "208", Mnc: "93"},
				AmfID:  amfID,
			})
			if err != nil {
				t.Fatal(err)
			}

			region, set, ptr := ngapConvert.AmfIdToNgap(amfID)

			if wantRegion := region.Bytes[0]; uint8(got.AMFRegionID) != wantRegion {
				t.Errorf("region = %#x, want %#x", got.AMFRegionID, wantRegion)
			}

			wantSet := uint16(set.Bytes[0])<<2 | uint16(set.Bytes[1])>>6
			if uint16(got.AMFSetID) != wantSet {
				t.Errorf("set = %#x, want %#x", got.AMFSetID, wantSet)
			}

			if wantPtr := ptr.Bytes[0] >> 2; uint8(got.AMFPointer) != wantPtr {
				t.Errorf("pointer = %#x, want %#x", got.AMFPointer, wantPtr)
			}
		})
	}
}

func TestPLMNRoundTripMatchesFree5GC(t *testing.T) {
	for _, plmn := range []models.PlmnID{
		{Mcc: "208", Mnc: "93"},
		{Mcc: "001", Mnc: "01"},
		{Mcc: "310", Mnc: "260"},
	} {
		t.Run(plmn.Mcc+"-"+plmn.Mnc, func(t *testing.T) {
			got, err := util.PLMNToNGAP(plmn)
			if err != nil {
				t.Fatal(err)
			}

			want, err := util.PlmnIDToNgap(plmn)
			if err != nil {
				t.Fatal(err)
			}

			if string(got[:]) != string(want.Value) {
				t.Fatalf("encoded % x, want % x", got, want.Value)
			}

			if back := util.PLMNToModels(got); back != plmn {
				t.Errorf("round trip = %+v, want %+v", back, plmn)
			}
		})
	}
}

func TestPLMNToNGAPRejectsMalformed(t *testing.T) {
	for _, plmn := range []models.PlmnID{
		{Mcc: "20", Mnc: "93"},
		{Mcc: "208", Mnc: "9"},
		{Mcc: "208", Mnc: "9333"},
		{},
	} {
		if _, err := util.PLMNToNGAP(plmn); err == nil {
			t.Errorf("accepted malformed PLMN %+v", plmn)
		}
	}
}
