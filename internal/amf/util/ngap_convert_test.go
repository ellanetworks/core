// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestRANNodeIDToModels(t *testing.T) {
	tests := []struct {
		name      string
		kind      ngap.RANNodeIDKind
		value     uint32
		bits      int
		wantGNB   string
		wantNgENB string
		wantN3IWF string
	}{
		{name: "gNB 22 bits", kind: ngap.RANNodeIDGNB, value: 0x3fabcd, bits: 22, wantGNB: "feaf34"},
		{name: "gNB 24 bits", kind: ngap.RANNodeIDGNB, value: 0x000102, bits: 24, wantGNB: "000102"},
		{name: "gNB 28 bits", kind: ngap.RANNodeIDGNB, value: 0xabcdef1, bits: 28, wantGNB: "abcdef1"},
		{name: "gNB 32 bits", kind: ngap.RANNodeIDGNB, value: 0xdeadbeef, bits: 32, wantGNB: "deadbeef"},
		{name: "macro ng-eNB", kind: ngap.RANNodeIDMacroNgENB, value: 0xabcde, bits: 20, wantNgENB: "MacroNGeNB-abcde"},
		{name: "short macro ng-eNB", kind: ngap.RANNodeIDShortMacroNgENB, value: 0x3abcd, bits: 18, wantNgENB: "SMacroNGeNB-eaf34"},
		{name: "long macro ng-eNB", kind: ngap.RANNodeIDLongMacroNgENB, value: 0x1abcde, bits: 21, wantNgENB: "LMacroNGeNB-d5e6f0"},
		{name: "N3IWF", kind: ngap.RANNodeIDN3IWF, value: 0xbeef, bits: 16, wantN3IWF: "beef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.RANNodeIDToModels(ngap.GlobalRANNodeID{
				Kind:         tt.kind,
				PLMNIdentity: ngap.PLMNIdentity{0x02, 0xf8, 0x39},
				Value:        tt.value,
				Bits:         tt.bits,
			})

			if got.NgeNbID != tt.wantNgENB || got.N3IwfID != tt.wantN3IWF {
				t.Errorf("ng-eNB/N3IWF = %q/%q, want %q/%q", got.NgeNbID, got.N3IwfID, tt.wantNgENB, tt.wantN3IWF)
			}

			if (got.GNbID == nil) != (tt.wantGNB == "") {
				t.Fatalf("gNB id presence: got %+v, want %q", got.GNbID, tt.wantGNB)
			}

			if got.GNbID != nil &&
				(got.GNbID.GNBValue != tt.wantGNB || got.GNbID.BitLength != int32(tt.bits)) {
				t.Errorf("gNB id = %+v, want %q/%d bits", *got.GNbID, tt.wantGNB, tt.bits)
			}
		})
	}
}

// GUAMIToNGAP splits the AMF id into the three NGAP fields, which is what
// decides the GUAMI a gNB sees in NG Setup Response. Expectations produced
// out-of-band as above.
func TestGUAMIToNGAP(t *testing.T) {
	tests := []struct {
		amfID      string
		wantRegion uint8
		wantSet    uint16
		wantPtr    uint8
	}{
		{"000000", 0x00, 0x000, 0x00},
		{"cafe00", 0xca, 0x3f8, 0x00},
		{"020040", 0x02, 0x001, 0x00},
		{"ffffff", 0xff, 0x3ff, 0x3f},
		{"123456", 0x12, 0x0d1, 0x16},
	}

	for _, tt := range tests {
		t.Run(tt.amfID, func(t *testing.T) {
			got, err := util.GUAMIToNGAP(models.Guami{
				PlmnID: &models.PlmnID{Mcc: "208", Mnc: "93"},
				AmfID:  tt.amfID,
			})
			if err != nil {
				t.Fatal(err)
			}

			if uint8(got.AMFRegionID) != tt.wantRegion {
				t.Errorf("region = %#x, want %#x", got.AMFRegionID, tt.wantRegion)
			}

			if uint16(got.AMFSetID) != tt.wantSet {
				t.Errorf("set = %#x, want %#x", got.AMFSetID, tt.wantSet)
			}

			if uint8(got.AMFPointer) != tt.wantPtr {
				t.Errorf("pointer = %#x, want %#x", got.AMFPointer, tt.wantPtr)
			}
		})
	}
}

// The BCD encoding of a PLMN differs between two- and three-digit MNCs
// (TS 38.413 §9.3.3.5), and a wrong filler nibble makes a gNB unreachable.
// Expectations produced out-of-band as above.
func TestPLMNRoundTrip(t *testing.T) {
	tests := []struct {
		plmn models.PlmnID
		want ngap.PLMNIdentity
	}{
		{models.PlmnID{Mcc: "208", Mnc: "93"}, ngap.PLMNIdentity{0x02, 0xf8, 0x39}},
		{models.PlmnID{Mcc: "001", Mnc: "01"}, ngap.PLMNIdentity{0x00, 0xf1, 0x10}},
		{models.PlmnID{Mcc: "310", Mnc: "260"}, ngap.PLMNIdentity{0x13, 0x20, 0x06}},
	}

	for _, tt := range tests {
		t.Run(tt.plmn.Mcc+"-"+tt.plmn.Mnc, func(t *testing.T) {
			got, err := util.PLMNToNGAP(tt.plmn)
			if err != nil {
				t.Fatal(err)
			}

			if got != tt.want {
				t.Fatalf("encoded %x, want %x", got, tt.want)
			}

			if back := util.PLMNToModels(got); back != tt.plmn {
				t.Errorf("round trip = %+v, want %+v", back, tt.plmn)
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
