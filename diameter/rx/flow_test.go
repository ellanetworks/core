// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"net/netip"
	"testing"
)

func TestParseFlowDescription(t *testing.T) {
	for s, want := range map[string]FlowDescription{
		"permit out 17 from 10.4.128.21 30000 to 192.168.101.4 1234": {
			Direction: DirectionOut, Protocol: ProtocolUDP,
			Source: netip.MustParsePrefix("10.4.128.21/32"), SourcePort: 30000,
			Destination: netip.MustParsePrefix("192.168.101.4/32"), DestinationPort: 1234,
		},
		"permit in ip from 192.168.101.5 5060 to 10.4.128.21 5060": {
			Direction: DirectionIn, Protocol: ProtocolIP,
			Source: netip.MustParsePrefix("192.168.101.5/32"), SourcePort: 5060,
			Destination: netip.MustParsePrefix("10.4.128.21/32"), DestinationPort: 5060,
		},
		"permit out 6 from 2001:db8::/64 to 2001:db8:1::1": {
			Direction: DirectionOut, Protocol: ProtocolTCP,
			Source: netip.MustParsePrefix("2001:db8::/64"), Destination: netip.MustParsePrefix("2001:db8:1::1/128"),
		},
		"permit in 17 from any to 10.0.0.1 5000": {
			Direction: DirectionIn, Protocol: ProtocolUDP, Destination: netip.MustParsePrefix("10.0.0.1/32"), DestinationPort: 5000,
		},
		"permit out 0 from any to any": {Direction: DirectionOut},
	} {
		got, err := ParseFlowDescription(s)
		if err != nil || got != want {
			t.Errorf("ParseFlowDescription(%q) = %+v, %v", s, got, err)
		}

		if got.String() != s {
			t.Errorf("String() = %q, want %q", got.String(), s)
		}
	}

	got, err := ParseFlowDescription("  permit  out 17 from 10.0.0.1/32 1000 to 10.0.0.2/32   ")
	if err != nil || got.String() != "permit out 17 from 10.0.0.1 1000 to 10.0.0.2" {
		t.Errorf("loose spacing = %q, %v", got, err)
	}
}

func TestParseFlowDescriptionRestrictions(t *testing.T) {
	for _, s := range []string{
		"",
		"deny out 17 from 10.0.0.1 to 10.0.0.2",
		"permit both 17 from 10.0.0.1 to 10.0.0.2",
		"permit out udp from 10.0.0.1 to 10.0.0.2",
		"permit out 256 from 10.0.0.1 to 10.0.0.2",
		"permit out 17 to 10.0.0.2",
		"permit out 17 from 10.0.0.1 10.0.0.2",
		"permit out 17 from 10.0.0.1 1000",
		"permit out 17 from !10.0.0.1 to 10.0.0.2",
		"permit out 17 from assigned to 10.0.0.2",
		"permit out 17 from 10.0.0.1 1000-2000 to 10.0.0.2",
		"permit out 17 from 10.0.0.1 1000,1001 to 10.0.0.2",
		"permit out 17 from 10.0.0.1 0 to 10.0.0.2",
		"permit out 17 from 10.0.0.1 to 10.0.0.2 70000",
		"permit out 17 from 10.0.0.1 to 10.0.0.2 2000 frag",
		"permit out 17 from fe80::1%eth0 to fe80::2",
		"permit out 17 from 10.0.0.1/33 to 10.0.0.2",
		"permit out 17 from 10.0.0.1 to",
	} {
		if _, err := ParseFlowDescription(s); !errors.Is(err, ErrInvalidFlowDescription) {
			t.Errorf("ParseFlowDescription(%q) = %v", s, err)
		}
	}
}

func TestFlowDescriptionMarshalText(t *testing.T) {
	for name, f := range map[string]FlowDescription{
		"direction": {Direction: 2},
		"protocol":  {Protocol: 257},
	} {
		if _, err := f.MarshalText(); !errors.Is(err, ErrInvalidFlowDescription) {
			t.Errorf("%s: err = %v", name, err)
		}

		if f.String() == "" {
			t.Errorf("%s: empty String()", name)
		}
	}
}
