// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func plmn() *models.PlmnID {
	return &models.PlmnID{Mcc: "001", Mnc: "01"}
}

func TestGlobalRanNodeIDString(t *testing.T) {
	tests := []struct {
		name string
		id   models.GlobalRanNodeID
		want string
	}{
		{
			"gNB carries its bit length",
			models.GlobalRanNodeID{PlmnID: plmn(), GNbID: &models.GNbID{GNBValue: "00002a", BitLength: 22}},
			"gNB/001-01/00002a@22",
		},
		{
			"ng-eNB",
			models.GlobalRanNodeID{PlmnID: plmn(), NgeNbID: "SMacroNGeNB-34b89"},
			"ng-eNB/001-01/SMacroNGeNB-34b89",
		},
		{
			"eNB",
			models.GlobalRanNodeID{PlmnID: plmn(), ENbID: "MacroeNB-00008"},
			"eNB/001-01/MacroeNB-00008",
		},
		{
			"N3IWF",
			models.GlobalRanNodeID{PlmnID: plmn(), N3IwfID: "beef"},
			"N3IWF/001-01/beef",
		},
		{
			"SNPN reports its NID",
			models.GlobalRanNodeID{PlmnID: plmn(), Nid: "000000000ab", ENbID: "MacroeNB-00008"},
			"eNB/001-01/000000000ab/MacroeNB-00008",
		},
		{
			"a missing PLMN is visible, not silently dropped",
			models.GlobalRanNodeID{ENbID: "MacroeNB-00008"},
			"eNB/-/MacroeNB-00008",
		},
		{
			"no alternative",
			models.GlobalRanNodeID{PlmnID: plmn()},
			"Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGlobalRanNodeIDDistinguishesEveryLeg(t *testing.T) {
	ids := []models.GlobalRanNodeID{
		{PlmnID: plmn(), GNbID: &models.GNbID{GNBValue: "00002a", BitLength: 22}},
		{PlmnID: plmn(), GNbID: &models.GNbID{GNBValue: "00002a", BitLength: 24}},
		{PlmnID: &models.PlmnID{Mcc: "208", Mnc: "93"}, GNbID: &models.GNbID{GNBValue: "00002a", BitLength: 22}},
		{PlmnID: plmn(), Nid: "000000000ab", GNbID: &models.GNbID{GNBValue: "00002a", BitLength: 22}},
		{PlmnID: plmn(), ENbID: "MacroeNB-00008"},
		{PlmnID: plmn(), ENbID: "HomeeNB-0000008"},
		{PlmnID: plmn(), NgeNbID: "MacroNGeNB-00008"},
	}

	keys := make(map[string]string, len(ids))

	for _, id := range ids {
		key, ok := id.Key()
		if !ok {
			t.Fatalf("%s has no key", id)
		}

		if prev, clash := keys[key]; clash {
			t.Errorf("%s and %s share key %q", prev, id, key)
		}

		keys[key] = id.String()
	}

	if len(keys) != len(ids) {
		t.Errorf("%d identities produced %d keys", len(ids), len(keys))
	}
}
