// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestRegistrationRequestCarriesTheAdditionalGUTI covers TS 24.501 §8.2.6.12 a):
// a single-registration UE registering after an inter-system change from S1 mode
// puts its native 5G-GUTI in the Additional GUTI while the message's own 5GS
// mobile identity carries the one mapped from the 4G-GUTI. Only the additional
// one names a context the AMF may still hold (§5.5.1.3.2 case e), so discarding
// it — which is what the codec did before — loses the native context.
func TestRegistrationRequestCarriesTheAdditionalGUTI(t *testing.T) {
	native := GUTIIdentity(GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 0x01, AMFSetID: 0x002, AMFPointer: 0x03,
		TMSI: [4]byte{1, 2, 3, 4},
	})
	mapped := GUTIIdentity(GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 0x11, AMFSetID: 0x012, AMFPointer: 0x13,
		TMSI: [4]byte{0x11, 0x12, 0x13, 0x14},
	})

	m := &RegistrationRequest{
		RegistrationType: RegistrationTypeMobilityUpdating,
		NgKSI:            nas.KeySetIdentifier{Value: 1, Mapped: true},
		MobileIdentity:   mapped,
		AdditionalGUTI:   &native,
		UEStatus:         &UEStatus{S1ModeReg: true},
	}

	raw, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParseRegistrationRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(back.AdditionalGUTI, &native) {
		t.Errorf("additional GUTI = %+v, want %+v", back.AdditionalGUTI, &native)
	}

	if !reflect.DeepEqual(back.MobileIdentity, mapped) {
		t.Errorf("5GS mobile identity = %+v, want %+v", back.MobileIdentity, mapped)
	}

	if back.UEStatus == nil || !back.UEStatus.S1ModeReg {
		t.Errorf("UE status = %v, want EMM-REGISTERED", back.UEStatus)
	}

	if len(back.Unrecognized) != 0 {
		t.Errorf("unrecognized = %+v, want none", back.Unrecognized)
	}
}
