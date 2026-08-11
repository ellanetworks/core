// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §5.5.1.2.4, §9.9.3.12A
func TestNetworkFeatureSupportNeverAdvertisesInterworkingWithoutN26(t *testing.T) {
	m := &MME{}

	if nfs := m.NetworkFeatureSupport(eps.UENetworkCapability{}); nfs.IWKN26 {
		t.Error("IWK N26 advertised to a UE that did not indicate N1 mode")
	}

	// Octet 9 bit 6 is N1 mode; octet 8 bit 8 is ePCO.
	nfs := m.NetworkFeatureSupport(eps.UENetworkCapability{Rest: []byte{0x00, 0x80, 0x20}})
	if nfs.IWKN26 {
		t.Error("IWK N26 advertised by an MME that supports N26")
	}

	if !nfs.EPCO {
		t.Error("ePCO not advertised to a UE that indicated support for the IE")
	}

	if !nfs.IMSVoPS {
		t.Error("the IMS VoPS indication was lost")
	}

	if nfs := m.NetworkFeatureSupport(eps.UENetworkCapability{Rest: []byte{0x00, 0x00, 0x20}}); nfs.EPCO {
		t.Error("ePCO advertised to a UE that did not indicate support for the IE")
	}
}

func TestUENetworkCapabilitySupportsEPCO(t *testing.T) {
	for _, tc := range []struct {
		name string
		rest []byte
		want bool
	}{
		{"no feature octets at all", nil, false},
		{"too short to reach octet 8", []byte{0x00}, false},
		{"octet 8 present, ePCO clear", []byte{0x00, 0x00}, false},
		{"octet 8 present, ePCO set", []byte{0x00, 0x80}, true},
		{"HC-CP CIoT set, ePCO clear", []byte{0x00, 0x40}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := eps.UENetworkCapability{Rest: tc.rest}
			if got := c.SupportsEPCO(); got != tc.want {
				t.Errorf("SupportsEPCO() = %v, want %v", got, tc.want)
			}
		})
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
