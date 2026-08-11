// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func fiveGSTargetPLMN(t *testing.T, plmn models.PlmnID) s1ap.PLMNIdentity {
	t.Helper()

	encoded, err := EncodePLMN(plmn)
	if err != nil {
		t.Fatalf("EncodePLMN(%+v): %v", plmn, err)
	}

	return encoded
}

func TestNGRANIdentityFromS1APGNB(t *testing.T) {
	nodePLMN := models.PlmnID{Mcc: "001", Mnc: "01"}
	taiPLMN := models.PlmnID{Mcc: "999", Mnc: "070"}

	target := s1ap.TargetNgRanNodeID{
		GlobalRANNodeID: s1ap.GlobalRANNodeID{GNB: &s1ap.GNB{GlobalGNBID: s1ap.GlobalGNBID{
			PLMNIdentity: fiveGSTargetPLMN(t, nodePLMN),
			GNBID:        s1ap.GNBID{Value: 0x1a2b3c, Bits: 24},
		}}},
		SelectedTAI: s1ap.FiveGSTAI{
			PLMNIdentity: fiveGSTargetPLMN(t, taiPLMN),
			TAC:          0xabcdef,
		},
	}

	got, err := NGRANIdentityFromS1AP(target)
	if err != nil {
		t.Fatalf("NGRANIdentityFromS1AP: %v", err)
	}

	want := interworking.NGRANIdentity{
		Kind:   interworking.NGRANNodeGNB,
		PlmnID: nodePLMN,
		ID:     0x1a2b3c,
		Bits:   24,
		SelectedTAI: interworking.FiveGSTAI{
			PlmnID: taiPLMN,
			TAC:    0xabcdef,
		},
	}

	if got != want {
		t.Fatalf("identity = %+v, want %+v", got, want)
	}
}

func TestNGRANIdentityFromS1APNgENB(t *testing.T) {
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}
	encoded := fiveGSTargetPLMN(t, plmn)

	for _, tc := range []struct {
		name string
		kind s1ap.ENBIDKind
		bits uint8
	}{
		{"short macro", s1ap.ENBIDShortMacro, 18},
		{"macro", s1ap.ENBIDMacro, 20},
		{"long macro", s1ap.ENBIDLongMacro, 21},
		{"home", s1ap.ENBIDHome, 28},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := s1ap.TargetNgRanNodeID{
				GlobalRANNodeID: s1ap.GlobalRANNodeID{NgENB: &s1ap.NgENB{GlobalNgENBID: s1ap.GlobalENBID{
					PLMNIdentity: encoded,
					ENBID:        s1ap.ENBID{Kind: tc.kind, Value: 0x201},
				}}},
				SelectedTAI: s1ap.FiveGSTAI{PLMNIdentity: encoded, TAC: 1},
			}

			got, err := NGRANIdentityFromS1AP(target)
			if err != nil {
				t.Fatalf("NGRANIdentityFromS1AP: %v", err)
			}

			want := interworking.NGRANIdentity{
				Kind:        interworking.NGRANNodeNgENB,
				PlmnID:      plmn,
				ID:          0x201,
				Bits:        tc.bits,
				SelectedTAI: interworking.FiveGSTAI{PlmnID: plmn, TAC: 1},
			}

			if got != want {
				t.Fatalf("identity = %+v, want %+v", got, want)
			}
		})
	}
}

func TestNGRANIdentityFromS1APRejectsUnusableTargets(t *testing.T) {
	plmn := fiveGSTargetPLMN(t, models.PlmnID{Mcc: "001", Mnc: "01"})

	for _, tc := range []struct {
		name string
		node s1ap.GlobalRANNodeID
	}{
		{"no alternative", s1ap.GlobalRANNodeID{}},
		{"gNB identity too narrow", s1ap.GlobalRANNodeID{GNB: &s1ap.GNB{GlobalGNBID: s1ap.GlobalGNBID{
			PLMNIdentity: plmn,
			GNBID:        s1ap.GNBID{Value: 1, Bits: 21},
		}}}},
		{"gNB identity too wide", s1ap.GlobalRANNodeID{GNB: &s1ap.GNB{GlobalGNBID: s1ap.GlobalGNBID{
			PLMNIdentity: plmn,
			GNBID:        s1ap.GNBID{Value: 1, Bits: 33},
		}}}},
		{"unknown ng-eNB identity kind", s1ap.GlobalRANNodeID{NgENB: &s1ap.NgENB{GlobalNgENBID: s1ap.GlobalENBID{
			PLMNIdentity: plmn,
			ENBID:        s1ap.ENBID{Kind: s1ap.ENBIDKind(9), Value: 1},
		}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NGRANIdentityFromS1AP(s1ap.TargetNgRanNodeID{
				GlobalRANNodeID: tc.node,
				SelectedTAI:     s1ap.FiveGSTAI{PLMNIdentity: plmn, TAC: 1},
			})
			if !errors.Is(err, ErrUnusableTargetNGRAN) {
				t.Fatalf("error = %v, want ErrUnusableTargetNGRAN", err)
			}
		})
	}
}
