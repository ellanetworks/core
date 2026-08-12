// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestNGRANIdentityToNGAP(t *testing.T) {
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}

	encoded, err := util.PLMNToNGAP(plmn)
	if err != nil {
		t.Fatalf("PLMNToNGAP: %v", err)
	}

	for _, tc := range []struct {
		name string
		in   interworking.NGRANIdentity
		want ngap.GlobalRANNodeID
	}{
		{
			name: "gNB",
			in:   interworking.NGRANIdentity{Kind: interworking.NGRANNodeGNB, PlmnID: plmn, ID: 0x1a2b3c, Bits: 24},
			want: ngap.GlobalRANNodeID{Kind: ngap.RANNodeIDGNB, PLMNIdentity: encoded, Value: 0x1a2b3c, Bits: 24},
		},
		{
			name: "short macro ng-eNB",
			in:   interworking.NGRANIdentity{Kind: interworking.NGRANNodeNgENB, PlmnID: plmn, ID: 0x201, Bits: 18},
			want: ngap.GlobalRANNodeID{Kind: ngap.RANNodeIDShortMacroNgENB, PLMNIdentity: encoded, Value: 0x201, Bits: 18},
		},
		{
			name: "macro ng-eNB",
			in:   interworking.NGRANIdentity{Kind: interworking.NGRANNodeNgENB, PlmnID: plmn, ID: 0x201, Bits: 20},
			want: ngap.GlobalRANNodeID{Kind: ngap.RANNodeIDMacroNgENB, PLMNIdentity: encoded, Value: 0x201, Bits: 20},
		},
		{
			name: "long macro ng-eNB",
			in:   interworking.NGRANIdentity{Kind: interworking.NGRANNodeNgENB, PlmnID: plmn, ID: 0x201, Bits: 21},
			want: ngap.GlobalRANNodeID{Kind: ngap.RANNodeIDLongMacroNgENB, PLMNIdentity: encoded, Value: 0x201, Bits: 21},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := amf.NGRANIdentityToNGAP(tc.in)
			if err != nil {
				t.Fatalf("NGRANIdentityToNGAP: %v", err)
			}

			if got != tc.want {
				t.Fatalf("node id = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNGRANIdentityToNGAPRejectsWidthsNGAPCannotCarry(t *testing.T) {
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}

	for _, tc := range []struct {
		name string
		in   interworking.NGRANIdentity
	}{
		{"gNB narrower than 22 bits", interworking.NGRANIdentity{Kind: interworking.NGRANNodeGNB, PlmnID: plmn, Bits: 21}},
		{"gNB wider than 32 bits", interworking.NGRANIdentity{Kind: interworking.NGRANNodeGNB, PlmnID: plmn, Bits: 33}},
		{"home ng-eNB", interworking.NGRANIdentity{Kind: interworking.NGRANNodeNgENB, PlmnID: plmn, Bits: 28}},
		{"unknown kind", interworking.NGRANIdentity{Kind: interworking.NGRANNodeKind(7), PlmnID: plmn, Bits: 24}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := amf.NGRANIdentityToNGAP(tc.in); err == nil {
				t.Fatal("the identity was accepted")
			}
		})
	}
}
