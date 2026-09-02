// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	naslib "github.com/ellanetworks/core/nas"
)

// TS 24.008 §10.5.6.3 scopes a container identifier to the direction: 000DH is
// a request uplink and the address itself downlink, so the same element must not
// be named the same way in both.
func TestExtendedPCONamesContainersPerDirection(t *testing.T) {
	for _, tc := range []struct {
		name      string
		direction naslib.PCODirection
		want      string
		wantDir   string
		// the flag form this replaced recorded presence only, so the contents
		// must survive: read downlink, kept as bytes uplink where the identifier
		// means an empty request and the spec says to ignore any contents
		wantValue string
		wantHex   string
	}{
		{"uplink", naslib.PCOMSToNetwork, "DNS Server IPv4 Address Request", "uplink", "", "08080808"},
		{"downlink", naslib.PCONetworkToMS, "DNS Server IPv4 Address", "downlink", "8.8.8.8", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := ExtendedPCO(&naslib.ProtocolConfigurationOptions{
				Direction:  tc.direction,
				Containers: []naslib.PCOContainer{{ID: 0x000D, Content: []byte{0x08, 0x08, 0x08, 0x08}}},
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

			if got := out.Containers[0].Value; got != tc.wantValue {
				t.Errorf("value = %q, want %q", got, tc.wantValue)
			}

			if got := out.Containers[0].Hex; got != tc.wantHex {
				t.Errorf("hex = %q, want %q", got, tc.wantHex)
			}
		})
	}
}

func TestExtendedPCOAbsentStaysNil(t *testing.T) {
	if got := ExtendedPCO(nil); got != nil {
		t.Fatalf("expected nil for an absent element, got %+v", got)
	}
}

// TS 24.008 §10.5.6.3 gives these containers a fixed width and meaning, so their
// contents are read rather than left as hex. Uplink the same identifiers are
// empty requests, so nothing is read there.
func TestExtendedPCOReadsFixedWidthContents(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   uint16
		in   []byte
		want string
	}{
		{"dns server ipv4 address", 0x000D, []byte{8, 8, 8, 8}, "8.8.8.8"},
		{"p-cscf ipv4 address", 0x000C, []byte{10, 0, 0, 1}, "10.0.0.1"},
		{"dns server ipv6 address", 0x0003, []byte{0x20, 0x01, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}, "2001:4860::8888"},
		{"ipv4 link mtu", 0x0010, []byte{0x05, 0x78}, "1400"},
		{"selected bearer control mode", 0x0005, []byte{0x02}, "MS/NW"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := ExtendedPCO(&naslib.ProtocolConfigurationOptions{
				Direction:  naslib.PCONetworkToMS,
				Containers: []naslib.PCOContainer{{ID: tc.id, Content: tc.in}},
			})

			if got := out.Containers[0].Value; got != tc.want {
				t.Errorf("value = %q, want %q", got, tc.want)
			}

			// a value that was read replaces the bytes, as every other decoded
			// value in the decoders does
			if h := out.Containers[0].Hex; h != "" {
				t.Errorf("hex = %q, want none once the value was read", h)
			}
		})
	}
}

// 0017H carries the UE's status uplink and is an empty support indication
// downlink, the reverse of the address containers.
func TestExtendedPCOReadsUplinkOnlyContents(t *testing.T) {
	for _, tc := range []struct {
		name      string
		direction naslib.PCODirection
		want      string
	}{
		{"uplink carries the status", naslib.PCOMSToNetwork, "deactivated"},
		{"downlink carries nothing", naslib.PCONetworkToMS, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := ExtendedPCO(&naslib.ProtocolConfigurationOptions{
				Direction:  tc.direction,
				Containers: []naslib.PCOContainer{{ID: 0x0017, Content: []byte{0x01}}},
			})

			if got := out.Containers[0].Value; got != tc.want {
				t.Errorf("value = %q, want %q", got, tc.want)
			}
		})
	}
}

// 0014H means the same thing in both directions.
func TestExtendedPCOReadsNBIFOMModeEitherDirection(t *testing.T) {
	for _, dir := range []naslib.PCODirection{naslib.PCOMSToNetwork, naslib.PCONetworkToMS} {
		out := ExtendedPCO(&naslib.ProtocolConfigurationOptions{
			Direction:  dir,
			Containers: []naslib.PCOContainer{{ID: 0x0014, Content: []byte{0x01}}},
		})

		if got := out.Containers[0].Value; got != "network-initiated" {
			t.Errorf("value = %q, want network-initiated", got)
		}
	}
}

func TestExtendedPCOKeepsHexWhenTheWidthIsWrong(t *testing.T) {
	out := ExtendedPCO(&naslib.ProtocolConfigurationOptions{
		Direction:  naslib.PCONetworkToMS,
		Containers: []naslib.PCOContainer{{ID: 0x000D, Content: []byte{1, 2, 3}}},
	})

	if v := out.Containers[0].Value; v != "" {
		t.Errorf("value = %q, want none for a three octet IPv4 address", v)
	}

	if out.Containers[0].Hex != "010203" {
		t.Errorf("hex = %q, want the bytes kept when they could not be read", out.Containers[0].Hex)
	}
}

func TestExtendedPCOLeavesOtherProtocolsAsHex(t *testing.T) {
	out := ExtendedPCO(&naslib.ProtocolConfigurationOptions{
		Direction:  naslib.PCONetworkToMS,
		Containers: []naslib.PCOContainer{{ID: 0xC023, Content: []byte{0x01, 0x02}}},
	})

	if v := out.Containers[0].Value; v != "" {
		t.Errorf("value = %q, want none for a PPP payload", v)
	}

	if out.Containers[0].Hex != "0102" {
		t.Errorf("hex = %q, want the bytes kept", out.Containers[0].Hex)
	}
}

// The identifier means an empty request uplink, so there is nothing to read.
func TestExtendedPCOReadsNothingUplink(t *testing.T) {
	out := ExtendedPCO(&naslib.ProtocolConfigurationOptions{
		Direction:  naslib.PCOMSToNetwork,
		Containers: []naslib.PCOContainer{{ID: 0x000D, Content: []byte{8, 8, 8, 8}}},
	})

	if v := out.Containers[0].Value; v != "" {
		t.Errorf("value = %q, want none uplink", v)
	}
}
