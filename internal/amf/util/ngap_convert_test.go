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
// (TS 38.413 §9.3.3.5, TS 24.008 §10.5.1.3 figure 10.5.3: octet 2 is
// MCC2|MCC1, octet 3 is MNC3|MCC3, octet 4 is MNC2|MNC1), and a wrong nibble
// order makes a gNB unreachable. The decode leg is checked against the literal
// vector rather than the encoder's output: a rotation round-trips with itself.
// Expectations produced out-of-band as above.
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

			if back := util.PLMNToModels(tt.want); back != tt.plmn {
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

// TS 38.413 §9.3.1.86: "The Security Capabilities received from NAS signaling
// shall not be modified or truncated when forwarded to NG-RAN nodes". Each
// bitmap gives its first three bits to 128-xxx1..3 and maps the fourth to
// seventh from bit 4 down to bit 1 of the matching TS 24.501 §9.11.3.54 octet,
// so algorithm n lands at 1<<(16-n) and identity 0 has no position.
func TestSecurityCapabilitiesToNGAP(t *testing.T) {
	tests := []struct {
		name string
		in   *fgs.UESecurityCapability
		want ngap.UESecurityCapabilities
	}{
		{
			name: "nil capability",
			in:   nil,
			want: ngap.UESecurityCapabilities{},
		},
		{
			name: "the null algorithms alone leave every bitmap zero",
			in:   &fgs.UESecurityCapability{EA: nas.Algorithms(0), IA: nas.Algorithms(0)},
			want: ngap.UESecurityCapabilities{},
		},
		{
			name: "5G-EA1 and 5G-IA2 take the first and second bits",
			in:   &fgs.UESecurityCapability{EA: nas.Algorithms(1), IA: nas.Algorithms(2)},
			want: ngap.UESecurityCapabilities{
				NREncryptionAlgorithms:          0x8000,
				NRIntegrityProtectionAlgorithms: 0x4000,
			},
		},
		{
			name: "the seventh algorithm reaches the seventh bit",
			in:   &fgs.UESecurityCapability{EA: nas.Algorithms(7), IA: nas.Algorithms(7)},
			want: ngap.UESecurityCapabilities{
				NREncryptionAlgorithms:          0x0200,
				NRIntegrityProtectionAlgorithms: 0x0200,
			},
		},
		{
			name: "every algorithm the bitmaps carry",
			in: &fgs.UESecurityCapability{
				EA: nas.Algorithms(0, 1, 2, 3, 4, 5, 6, 7),
				IA: nas.Algorithms(0, 1, 2, 3, 4, 5, 6, 7),
			},
			want: ngap.UESecurityCapabilities{
				NREncryptionAlgorithms:          0xfe00,
				NRIntegrityProtectionAlgorithms: 0xfe00,
			},
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
			want: ngap.UESecurityCapabilities{
				NREncryptionAlgorithms:             0xc000,
				NRIntegrityProtectionAlgorithms:    0x4000,
				EUTRAEncryptionAlgorithms:          0xa000,
				EUTRAIntegrityProtectionAlgorithms: 0x4000,
			},
		},
		{
			name: "a UE without S1 mode advertises no EPS algorithms",
			in: &fgs.UESecurityCapability{
				EA:  nas.Algorithms(1),
				IA:  nas.Algorithms(1),
				EEA: nas.Algorithms(1, 2, 3),
				EIA: nas.Algorithms(1, 2, 3),
			},
			want: ngap.UESecurityCapabilities{
				NREncryptionAlgorithms:          0x8000,
				NRIntegrityProtectionAlgorithms: 0x8000,
			},
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
