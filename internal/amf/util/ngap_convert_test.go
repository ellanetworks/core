// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

func TestRANNodeIDToModels(t *testing.T) {
	tests := []struct {
		name      string
		id        ngap.GlobalRANNodeID
		bits      int
		wantGNB   string
		wantNgENB string
		wantN3IWF string
	}{
		{name: "gNB 22 bits", id: ngap.NewGlobalGNBID(ngap.PLMNIdentity{0x02, 0xf8, 0x39}, 0x3fabcd, 22), bits: 22, wantGNB: "3fabcd"},
		{name: "gNB 24 bits", id: ngap.NewGlobalGNBID(ngap.PLMNIdentity{0x02, 0xf8, 0x39}, 0x000102, 24), bits: 24, wantGNB: "000102"},
		{name: "gNB 28 bits", id: ngap.NewGlobalGNBID(ngap.PLMNIdentity{0x02, 0xf8, 0x39}, 0xabcdef1, 28), bits: 28, wantGNB: "abcdef1"},
		{name: "gNB 32 bits", id: ngap.NewGlobalGNBID(ngap.PLMNIdentity{0x02, 0xf8, 0x39}, 0xdeadbeef, 32), bits: 32, wantGNB: "deadbeef"},
		{name: "macro ng-eNB", id: ngap.NewGlobalNgENBID(ngap.PLMNIdentity{0x02, 0xf8, 0x39}, ngap.NgENBID{MacroNgENBID: new(ngap.MacroNgENBID(0xabcde))}), bits: 20, wantNgENB: "MacroNGeNB-abcde"},
		{name: "short macro ng-eNB", id: ngap.NewGlobalNgENBID(ngap.PLMNIdentity{0x02, 0xf8, 0x39}, ngap.NgENBID{ShortMacroNgENBID: new(ngap.ShortMacroNgENBID(0x3abcd))}), bits: 18, wantNgENB: "SMacroNGeNB-3abcd"},
		{name: "long macro ng-eNB", id: ngap.NewGlobalNgENBID(ngap.PLMNIdentity{0x02, 0xf8, 0x39}, ngap.NgENBID{LongMacroNgENBID: new(ngap.LongMacroNgENBID(0x1abcde))}), bits: 21, wantNgENB: "LMacroNGeNB-1abcde"},
		{name: "N3IWF", id: ngap.NewGlobalN3IWFID(ngap.PLMNIdentity{0x02, 0xf8, 0x39}, 0xbeef), bits: 16, wantN3IWF: "beef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.RANNodeIDToModels(tt.id)
			if err != nil {
				t.Fatal(err)
			}

			if got.PlmnID == nil || got.PlmnID.Mcc != "208" || got.PlmnID.Mnc != "93" {
				t.Errorf("plmnId = %+v, want 208/93", got.PlmnID)
			}

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

// TS 24.008 §10.5.1.3
func TestPLMNRoundTrip(t *testing.T) {
	tests := []struct {
		plmn models.PlmnID
		want ngap.PLMNIdentity
	}{
		{models.PlmnID{Mcc: "208", Mnc: "93"}, ngap.PLMNIdentity{0x02, 0xf8, 0x39}},
		{models.PlmnID{Mcc: "001", Mnc: "01"}, ngap.PLMNIdentity{0x00, 0xf1, 0x10}},
		{models.PlmnID{Mcc: "310", Mnc: "260"}, ngap.PLMNIdentity{0x13, 0x00, 0x62}},
		{models.PlmnID{Mcc: "310", Mnc: "410"}, ngap.PLMNIdentity{0x13, 0x00, 0x14}},
		{models.PlmnID{Mcc: "302", Mnc: "720"}, ngap.PLMNIdentity{0x03, 0x02, 0x27}},
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

			back, err := util.PLMNToModels(tt.want)
			if err != nil {
				t.Fatal(err)
			}

			if back != tt.plmn {
				t.Errorf("decoded %+v, want %+v", back, tt.plmn)
			}

			octets, err := nas.PLMN{MCC: tt.plmn.Mcc, MNC: tt.plmn.Mnc}.Octets()
			if err != nil {
				t.Fatal(err)
			}

			if [3]byte(got) != octets {
				t.Errorf("NGAP %x and NAS %x disagree on the same PLMN", got, octets)
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

// TS 38.413 §9.3.1.86
func TestSecurityCapabilitiesToNGAP(t *testing.T) {
	tests := []struct {
		name string
		in   *fgs.UESecurityCapability
		want ngap.UESecurityCapabilities
	}{
		{
			name: "nil capability",
			in:   nil,
			want: secCaps(0, 0, 0, 0),
		},
		{
			name: "the null algorithms alone leave every bitmap zero",
			in:   &fgs.UESecurityCapability{EA: nas.Algorithms(0), IA: nas.Algorithms(0)},
			want: secCaps(0, 0, 0, 0),
		},
		{
			name: "5G-EA1 and 5G-IA2 take the first and second bits",
			in:   &fgs.UESecurityCapability{EA: nas.Algorithms(1), IA: nas.Algorithms(2)},
			want: secCaps(0x8000, 0x4000, 0, 0),
		},
		{
			name: "the seventh algorithm reaches the seventh bit",
			in:   &fgs.UESecurityCapability{EA: nas.Algorithms(7), IA: nas.Algorithms(7)},
			want: secCaps(0x0200, 0x0200, 0, 0),
		},
		{
			name: "every algorithm the bitmaps carry",
			in: &fgs.UESecurityCapability{
				EA: nas.Algorithms(0, 1, 2, 3, 4, 5, 6, 7),
				IA: nas.Algorithms(0, 1, 2, 3, 4, 5, 6, 7),
			},
			want: secCaps(0xfe00, 0xfe00, 0, 0),
		},
		{
			name: "a UE supporting S1 mode carries its EPS algorithms too",
			in: &fgs.UESecurityCapability{
				EA:     nas.Algorithms(1, 2),
				IA:     nas.Algorithms(2),
				HasEPS: true,
				EEA:    nas.Algorithms(1, 3),
				EIA:    nas.Algorithms(2),
			},
			want: secCaps(0xc000, 0x4000, 0xa000, 0x4000),
		},
		{
			name: "a UE without S1 mode advertises no EPS algorithms",
			in: &fgs.UESecurityCapability{
				EA:  nas.Algorithms(1),
				IA:  nas.Algorithms(1),
				EEA: nas.Algorithms(1, 2, 3),
				EIA: nas.Algorithms(1, 2, 3),
			},
			want: secCaps(0x8000, 0x8000, 0, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.SecurityCapabilitiesToNGAP(tt.in); got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func secCaps(nrEA, nrIA, eutraEA, eutraIA uint16) ngap.UESecurityCapabilities {
	return ngap.UESecurityCapabilities{
		NREncryptionAlgorithms:             ngap.SecurityAlgorithmsFrom(nrEA),
		NRIntegrityProtectionAlgorithms:    ngap.SecurityAlgorithmsFrom(nrIA),
		EUTRAEncryptionAlgorithms:          ngap.SecurityAlgorithmsFrom(eutraEA),
		EUTRAIntegrityProtectionAlgorithms: ngap.SecurityAlgorithmsFrom(eutraIA),
	}
}
