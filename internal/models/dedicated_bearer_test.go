// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

func TestEPSTFTPacketFilterComponents(t *testing.T) {
	for name, tc := range map[string]struct {
		filter models.SDFFilter
		want   []eps.TFTComponentType
	}{
		"IPv4 UDP with ports": {
			models.SDFFilter{Remote: netip.MustParsePrefix("192.0.2.10/32"), Protocol: 17, LocalPort: 50000, RemotePort: 49000},
			[]eps.TFTComponentType{eps.TFTIPv4RemoteAddress, eps.TFTProtocolIdentifier, eps.TFTSingleLocalPort, eps.TFTSingleRemotePort},
		},
		"IPv6 remote": {
			models.SDFFilter{Remote: netip.MustParsePrefix("2001:db8::10/128"), Protocol: 17, LocalPort: 50000},
			[]eps.TFTComponentType{eps.TFTIPv6RemoteAddress, eps.TFTProtocolIdentifier, eps.TFTSingleLocalPort},
		},
		"any remote, any protocol": {
			models.SDFFilter{LocalPort: 50000},
			[]eps.TFTComponentType{eps.TFTSingleLocalPort},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tft := models.EPSTFT(eps.TFTCreate, []models.SDFFilter{tc.filter})

			var got []eps.TFTComponentType
			for _, c := range tft.Filters[0].Components {
				got = append(got, c.Type)
			}

			if !slices.Equal(got, tc.want) {
				t.Fatalf("components %v, want %v (TS 24.008 §10.5.6.12)", got, tc.want)
			}
		})
	}
}

func TestGBRQCI(t *testing.T) {
	for qci, want := range map[uint8]bool{1: true, 2: true, 4: true, 5: false, 9: false, 65: true, 69: false, 75: true, 82: true, 86: false} {
		if got := models.GBRQCI(qci); got != want {
			t.Errorf("GBRQCI(%d) = %v, want %v (TS 23.203 Table 6.1.7)", qci, got, want)
		}
	}
}
