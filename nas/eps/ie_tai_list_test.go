// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestTAIListRoundTrip(t *testing.T) {
	cases := map[string]TAIList{
		"one partial list, shared PLMN": {
			{Type: PartialTAIListNonConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1}, {PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 2}}},
		},
		"consecutive run": {
			{Type: PartialTAIListConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 7}, {PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 8}, {PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 9}}},
		},
		"per-entry PLMN": {
			{Type: PartialTAIListPerPLMN, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1}, {PLMN: nas.PLMN{MCC: "999", MNC: "70"}, TAC: 0x0203}}},
		},
		"several partial lists": {
			{Type: PartialTAIListNonConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1}}},
			{Type: PartialTAIListPerPLMN, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "999", MNC: "70"}, TAC: 0xABCD}}},
		},
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			b, err := in.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary(%v): %v", in, err)
			}

			got, err := ParseTAIList(b)
			if err != nil {
				t.Fatalf("ParseTAIList(% x): %v", b, err)
			}

			if !reflect.DeepEqual(got, in) {
				t.Fatalf("round-trip = %+v, want %+v (wire % x)", got, in, b)
			}
		})
	}
}

// TestNewTAIListGroupsByPLMN checks the constructor packs a run sharing a PLMN
// into one partial list and starts a new one when the PLMN changes.
func TestNewTAIListGroupsByPLMN(t *testing.T) {
	tais := []TAI{
		{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1},
		{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 5},
		{PLMN: nas.PLMN{MCC: "999", MNC: "70"}, TAC: 9},
	}

	list, err := NewTAIList(tais...)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 2 || len(list[0].TAIs) != 2 || len(list[1].TAIs) != 1 {
		t.Fatalf("grouping = %+v, want two partial lists of 2 and 1", list)
	}

	if !reflect.DeepEqual(list.TAIs(), tais) {
		t.Fatalf("TAIs() = %+v, want %+v", list.TAIs(), tais)
	}

	for _, tai := range tais {
		if !list.Contains(tai) {
			t.Errorf("Contains(%s) = false", tai)
		}
	}

	if list.Contains(TAI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 2}) {
		t.Error("Contains reported an identity the list does not denote")
	}
}

// TestTAIListRejectsInconsistentPartialList checks the encoder refuses a partial
// list whose identities do not match the encoding its type declares.
func TestTAIListRejectsInconsistentPartialList(t *testing.T) {
	cases := map[string]TAIList{
		"shared-PLMN type with two PLMNs": {
			{Type: PartialTAIListNonConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1}, {PLMN: nas.PLMN{MCC: "999", MNC: "70"}, TAC: 2}}},
		},
		"consecutive type with a gap": {
			{Type: PartialTAIListConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 1}, {PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 3}}},
		},
		"consecutive run past the TAC width": {
			{Type: PartialTAIListConsecutive, TAIs: []TAI{{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 0xFFFF}, {PLMN: nas.PLMN{MCC: "001", MNC: "01"}, TAC: 0}}},
		},
		"empty partial list": {
			{Type: PartialTAIListNonConsecutive},
		},
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if b, err := in.MarshalBinary(); err == nil {
				t.Fatalf("MarshalBinary succeeded (% x), want an error", b)
			}
		})
	}
}
