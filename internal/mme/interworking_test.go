// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// The IWK N26 indication goes only to a UE that indicated N1 mode: one that
// cannot reach 5GS has nothing to do with it (TS 24.301 §5.5.1.2.4, §5.5.3.2.4).
// The bit reads inverted from most feature bits — 1 means the network has no N26
// interface, so the UE must move its own sessions.
func TestNetworkFeatureSupportAdvertisesInterworking(t *testing.T) {
	m := &MME{}

	if nfs := m.NetworkFeatureSupport(false); nfs.IWKN26 {
		t.Error("IWK N26 advertised to a UE that did not indicate N1 mode")
	}

	nfs := m.NetworkFeatureSupport(true)
	if !nfs.IWKN26 {
		t.Error("IWK N26 not advertised to a UE that indicated N1 mode")
	}

	// The rest of the element is untouched.
	if !nfs.IMSVoPS {
		t.Error("the IMS VoPS indication was lost")
	}
}

// N1 mode is octet 9 bit 6 of the UE network capability, which is Rest[2] since
// Rest starts at octet 7 (TS 24.301 figure 9.9.3.34.1). A capability too short
// to carry the octet indicated nothing, which §9.9.3.34 reads as all-zero.
func TestUENetworkCapabilitySupportsN1Mode(t *testing.T) {
	for _, tc := range []struct {
		name string
		rest []byte
		want bool
	}{
		{"no feature octets at all", nil, false},
		{"too short to reach octet 9", []byte{0x00, 0x00}, false},
		{"octet 9 present, N1 mode clear", []byte{0x00, 0x00, 0x00}, false},
		{"octet 9 present, N1 mode set", []byte{0x00, 0x00, 0x20}, true},
		// The neighbouring bits must not be mistaken for it: bit 5 is DCNR and
		// bit 7 is SGC.
		{"DCNR set, N1 mode clear", []byte{0x00, 0x00, 0x10}, false},
		{"SGC set, N1 mode clear", []byte{0x00, 0x00, 0x40}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := eps.UENetworkCapability{Rest: tc.rest}
			if got := c.SupportsN1Mode(); got != tc.want {
				t.Errorf("SupportsN1Mode() = %v, want %v", got, tc.want)
			}
		})
	}
}
