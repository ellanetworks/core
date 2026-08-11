// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestUEStatusWire(t *testing.T) {
	tests := []struct {
		name   string
		status UEStatus
		octet  byte
	}{
		{"neither", UEStatus{}, 0x00},
		{"EMM-REGISTERED", UEStatus{S1ModeReg: true}, 0x01},
		{"5GMM-REGISTERED", UEStatus{N1ModeReg: true}, 0x02},
		{"both", UEStatus{S1ModeReg: true, N1ModeReg: true}, 0x03},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if raw := tc.status.MarshalBinary(); !bytes.Equal(raw, []byte{tc.octet}) {
				t.Fatalf("UE status = % x, want %#02x", raw, tc.octet)
			}

			back, err := ParseUEStatus([]byte{tc.octet})
			if err != nil {
				t.Fatal(err)
			}

			if back != tc.status {
				t.Fatalf("round trip = %+v, want %+v", back, tc.status)
			}
		})
	}

	for _, b := range [][]byte{{}, {0x00, 0x00}} {
		if _, err := ParseUEStatus(b); err == nil {
			t.Errorf("%d octets: want an error, got none", len(b))
		}
	}
}

// TestTAURequestCarriesTheHandoverElements covers the three elements TS 24.301
// §5.5.3.2.2 case zd puts in the TAU that follows an inter-system handover from
// 5GS. Before this they were delimited but discarded, so the MME could not tell
// the move from an ordinary tracking area update.
func TestTAURequestCarriesTheHandoverElements(t *testing.T) {
	native := GUTIIdentity(GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 0x0102, MMECode: 0x03, TMSI: [4]byte{4, 5, 6, 7},
	})
	mapped := GUTIIdentity(GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 0x1112, MMECode: 0x13, TMSI: [4]byte{0x14, 0x15, 0x16, 0x17},
	})
	guti := GUTITypeNative
	status := UEStatus{N1ModeReg: true}

	m := &TrackingAreaUpdateRequest{
		EPSUpdateType:  EPSUpdateTypeTA,
		OldGUTI:        mapped,
		AdditionalGUTI: &native,
		OldGUTIType:    &guti,
		UEStatus:       &status,
	}

	raw, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParseTrackingAreaUpdateRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(back.AdditionalGUTI, &native) {
		t.Errorf("additional GUTI = %+v, want %+v", back.AdditionalGUTI, &native)
	}

	if !reflect.DeepEqual(back.OldGUTI, mapped) {
		t.Errorf("old GUTI = %+v, want %+v", back.OldGUTI, mapped)
	}

	if back.OldGUTIType == nil || *back.OldGUTIType != GUTITypeNative {
		t.Errorf("old GUTI type = %v, want native", back.OldGUTIType)
	}

	if back.UEStatus == nil || *back.UEStatus != status {
		t.Errorf("UE status = %v, want %+v", back.UEStatus, status)
	}

	if len(back.Unrecognized) != 0 {
		t.Errorf("unrecognized = %+v, want none", back.Unrecognized)
	}
}

// TestAttachRequestCarriesUEStatus is the ATTACH REQUEST half of the same
// element (TS 24.301 §8.2.4.22).
func TestAttachRequestCarriesUEStatus(t *testing.T) {
	status := UEStatus{N1ModeReg: true}

	m := &AttachRequest{
		EPSAttachType: AttachTypeEPS,
		EPSMobileIdentity: GUTIIdentity(GUTI{
			PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 0x0102, MMECode: 0x03, TMSI: [4]byte{4, 5, 6, 7},
		}),
		UEStatus: &status,
	}

	raw, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParseAttachRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	if back.UEStatus == nil || *back.UEStatus != status {
		t.Fatalf("UE status = %v, want %+v", back.UEStatus, status)
	}

	if len(back.Unrecognized) != 0 {
		t.Errorf("unrecognized = %+v, want none", back.Unrecognized)
	}
}
