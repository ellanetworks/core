// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package etsi_test

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 23.003 §2.10.2.1.2
func TestMapGUTI5GToEPS(t *testing.T) {
	plmn := nas.PLMN{MCC: "001", MNC: "01"}
	tmsi := [4]byte{0xde, 0xad, 0xbe, 0xef}

	for _, tc := range []struct {
		name       string
		in         fgs.GUTI
		wantGroup  uint16
		wantMMECod uint8
	}{
		{
			name:      "region id fills the top of the group id",
			in:        fgs.GUTI{PLMN: plmn, AMFRegionID: 0xAB, TMSI: tmsi},
			wantGroup: 0xAB00,
		},
		{
			name:      "set id's high 8 bits fill the bottom of the group id",
			in:        fgs.GUTI{PLMN: plmn, AMFSetID: 0x3FC, TMSI: tmsi}, // 0b11_1111_1100
			wantGroup: 0x00FF,
		},
		{
			name:       "set id's low 2 bits top the mme code",
			in:         fgs.GUTI{PLMN: plmn, AMFSetID: 0x003, TMSI: tmsi},
			wantMMECod: 0xC0,
		},
		{
			name:       "pointer fills the bottom of the mme code",
			in:         fgs.GUTI{PLMN: plmn, AMFPointer: 0x3F, TMSI: tmsi},
			wantMMECod: 0x3F,
		},
		{
			name:       "every field at once",
			in:         fgs.GUTI{PLMN: plmn, AMFRegionID: 0x01, AMFSetID: 0x008, AMFPointer: 0x03, TMSI: tmsi},
			wantGroup:  0x0102, // region 0x01, set id 0b00_0000_1000 >> 2 = 0x02
			wantMMECod: 0x03,   // set id low bits 0b00, pointer 0x03
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := etsi.MapGUTI5GToEPS(tc.in)

			if got.MMEGroupID != tc.wantGroup {
				t.Errorf("MME Group ID = %#04x, want %#04x", got.MMEGroupID, tc.wantGroup)
			}

			if got.MMECode != tc.wantMMECod {
				t.Errorf("MME Code = %#02x, want %#02x", got.MMECode, tc.wantMMECod)
			}

			// "5GS <5G-TMSI> maps to E-UTRAN <M-TMSI>" — unchanged.
			if got.TMSI != tc.in.TMSI {
				t.Errorf("M-TMSI = % x, want the 5G-TMSI % x", got.TMSI, tc.in.TMSI)
			}

			if got.PLMN != tc.in.PLMN {
				t.Errorf("PLMN = %v, want %v", got.PLMN, tc.in.PLMN)
			}
		})
	}
}

func TestMapGUTI5GToEPSMasksOverwideFields(t *testing.T) {
	got := etsi.MapGUTI5GToEPS(fgs.GUTI{AMFPointer: 0xFF})

	if got.MMECode != 0x3F {
		t.Errorf("MME Code = %#02x, want the pointer masked to six bits", got.MMECode)
	}
}
