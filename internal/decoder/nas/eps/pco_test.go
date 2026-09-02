// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/nas"
)

// TS 24.008 §10.5.6.3 scopes a container identifier to the direction: 000DH is
// a request uplink and the address itself downlink, so the same element must not
// be named the same way in both.
func TestExtendedPCONamesContainersPerDirection(t *testing.T) {
	for _, tc := range []struct {
		name      string
		direction nas.PCODirection
		want      string
		wantDir   string
	}{
		{"uplink", nas.PCOMSToNetwork, "DNS Server IPv4 Address Request", "uplink"},
		{"downlink", nas.PCONetworkToMS, "DNS Server IPv4 Address", "downlink"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := nasie.ExtendedPCO(&nas.ProtocolConfigurationOptions{
				Direction:  tc.direction,
				Containers: []nas.PCOContainer{{ID: 0x000D, Content: []byte{0x08, 0x08, 0x08, 0x08}}},
			})

			if len(out.Containers) != 1 {
				t.Fatalf("got %d containers, want 1", len(out.Containers))
			}

			if out.Containers[0].Name != tc.want {
				t.Errorf("name = %q, want %q", out.Containers[0].Name, tc.want)
			}

			if out.Direction != tc.wantDir {
				t.Errorf("direction = %q, want %q", out.Direction, tc.wantDir)
			}

			// the flag form this replaced recorded presence only
			if out.Containers[0].Hex != "08080808" {
				t.Errorf("hex = %q, want the container contents", out.Containers[0].Hex)
			}
		})
	}
}

func TestExtendedPCOAbsentStaysNil(t *testing.T) {
	if got := nasie.ExtendedPCO(nil); got != nil {
		t.Fatalf("expected nil for an absent element, got %+v", got)
	}
}
