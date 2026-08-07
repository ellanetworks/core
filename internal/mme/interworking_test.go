// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func TestNetworkFeatureSupportAdvertisesInterworking(t *testing.T) {
	m := &MME{}

	if nfs := m.NetworkFeatureSupport(false); nfs.IWKN26 {
		t.Error("IWK N26 advertised to a UE that did not indicate N1 mode")
	}

	nfs := m.NetworkFeatureSupport(true)
	if !nfs.IWKN26 {
		t.Error("IWK N26 not advertised to a UE that indicated N1 mode")
	}

	if !nfs.IMSVoPS {
		t.Error("the IMS VoPS indication was lost")
	}
}

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
