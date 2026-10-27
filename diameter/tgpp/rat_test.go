// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import "testing"

func TestRATTypeString(t *testing.T) {
	if s := RATEUTRAN.String(); s != "EUTRAN" {
		t.Errorf("EUTRAN = %q", s)
	}

	if s := RATType(9999).String(); s != "RATType(9999)" {
		t.Errorf("unknown = %q", s)
	}
}

func TestRATTypeKnown(t *testing.T) {
	for rat, want := range map[RATType][2]bool{
		RATWLAN:             {true, false},
		RATEUTRAN:           {true, true},
		RATNREnhancedRedCap: {true, true},
		1999:                {false, true},
		RATCDMA20001X:       {true, false},
		9999:                {false, false},
	} {
		if rat.Known() != want[0] || rat.ThreeGPP() != want[1] {
			t.Errorf("%v: Known = %v, ThreeGPP = %v", rat, rat.Known(), rat.ThreeGPP())
		}
	}
}
