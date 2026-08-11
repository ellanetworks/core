// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

// PriorityLevel is INTEGER (0..15) where NGAP's PriorityLevelARP is (1..15), so
// 0 ("spare") is a value this IE can carry and its NGAP counterpart cannot.
func TestAllocationAndRetentionPriorityRoundTrip(t *testing.T) {
	for _, level := range []uint8{0, 1, 15} {
		in := AllocationAndRetentionPriority{
			PriorityLevel:           level,
			PreemptionCapability:    PreemptionMayTrigger,
			PreemptionVulnerability: PreemptionPreemptable,
		}

		w := per.NewWriter()
		if err := in.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("level %d: %v", level, err)
		}

		out, err := unmarshalPERValue[AllocationAndRetentionPriority](perBytes(w))
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}

		if out != in {
			t.Fatalf("level %d: round trip %+v, want %+v", level, out, in)
		}
	}
}

// S1AP's GBR-QosInformation carries only the four bit rates; NGAP's adds
// notification control and packet loss rates.
func TestGBRQosInformationRoundTrip(t *testing.T) {
	in := GBRQosInformation{
		MaximumBitrateDL:    1000,
		MaximumBitrateUL:    2000,
		GuaranteedBitrateDL: 300,
		GuaranteedBitrateUL: 400,
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	out, err := unmarshalPERValue[GBRQosInformation](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if out != in {
		t.Fatalf("round trip %+v, want %+v", out, in)
	}
}

// The list bounds come from the S1AP-Constants module. A wrong bound is not
// always visible in a golden — neighbouring bounds often share a length
// determinant width — so the values are pinned literally here.
func TestS1APConstantsMatchSpec(t *testing.T) {
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"maxnoofERABs", maxnoofERABs, 256},
		{"maxnoofTAIs", maxnoofTAIs, 256},
		{"maxnoofTACs", maxnoofTACs, 256},
		{"maxnoofErrors", maxnoofErrors, 256},
		{"maxnoofBPLMNs", maxnoofBPLMNs, 6},
		{"maxnoofRATs", maxnoofRATs, 8},
		{"maxnoofPLMNsPerMME", maxnoofPLMNsPerMME, 32},
		{"maxnoofGroupIDs", maxnoofGroupIDs, 65535},
		{"maxnoofMMECs", maxnoofMMECs, 256},
		{"maxnoofEPLMNs", maxnoofEPLMNs, 15},
		{"maxnoofEPLMNsPlusOne", maxnoofEPLMNsPlusOne, 16},
		{"maxnoofForbTACs", maxnoofForbTACs, 4096},
		{"maxnoofForbLACs", maxnoofForbLACs, 4096},
	} {
		if c.got != c.want {
			t.Errorf("%s is %d, TS 36.413 S1AP-Constants says %d", c.name, c.got, c.want)
		}
	}
}
