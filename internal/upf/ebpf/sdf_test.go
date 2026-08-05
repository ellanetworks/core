// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"unsafe"
)

// TestSDFFilterEnforcement checks that uplink SDF rules drop denied traffic and
// pass everything else.
//
// A deny returns ActionDrop before routing. An allowed packet continues into
// the routing tail, which returns anything but ActionDrop absent blackhole or
// unreachable routes, and its inner packet must be intact.
func TestSDFFilterEnforcement(t *testing.T) {
	requireProgTestRun(t)

	const (
		filteredTEID   = 0x0A0B0C0D
		unfilteredTEID = 0x0A0B0C0E
		filterIndex    = 1
		dport          = 53
		protoUDP       = 17
	)

	dst := [4]byte{8, 8, 8, 8}

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, filteredTEID, filterIndex)
	putForwardingUplinkPDR(t, obj, unfilteredTEID, 0)

	deny := sdfRuleIPv4(dst, 32, dport, dport, protoUDP, SdfActionDeny)
	allow := sdfRuleIPv4(dst, 32, dport, dport, protoUDP, SdfActionAllow)
	denyOther := sdfRuleIPv4([4]byte{1, 1, 1, 1}, 32, dport, dport, protoUDP, SdfActionDeny)

	tests := []struct {
		name     string
		teid     uint32
		rules    []SdfRule
		wantDrop bool
	}{
		{"matching deny rule drops", filteredTEID, []SdfRule{deny}, true},
		{"matching allow rule passes", filteredTEID, []SdfRule{allow}, false},
		{"non-matching rule defaults to allow", filteredTEID, []SdfRule{denyOther}, false},
		{"empty filter list defaults to allow", filteredTEID, nil, false},
		{"no filter index passes", unfilteredTEID, nil, false},
	}

	inner := innerIPv4UDP(dst, dport)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.teid == filteredTEID {
				putSDFFilter(t, obj, filterIndex, tc.rules)
			}

			action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(tc.teid, inner))

			if tc.wantDrop {
				if action != ActionDrop {
					t.Fatalf("denied packet: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
				}

				return
			}

			if action == ActionDrop {
				t.Fatal("allowed packet was dropped")
			}

			if len(out) != ethHdrLen+len(inner) || !bytes.Equal(out[ethHdrLen:], inner) {
				t.Fatalf("allowed packet not decapsulated to its inner packet:\n got %x\nwant %x", out, inner)
			}
		})
	}
}

// TestSDFRuleMatching exercises the uplink SDF rule-matching dimensions:
// protocol (wildcard/match/mismatch), port range, address prefix (CIDR and
// wildcard), and first-match ordering. Deny => ActionDrop; allow => forwarded with
// the inner packet intact.
func TestSDFRuleMatching(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444601
		filterIndex = 1
		protoTCP    = 6
		protoUDP    = 17
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, filterIndex)

	dst := [4]byte{8, 8, 8, 8}
	udp53 := innerIPv4UDP(dst, 53)
	tcp80 := innerIPv4TCP(dst, 80)

	tests := []struct {
		name     string
		rules    []SdfRule
		inner    []byte
		wantDrop bool
	}{
		{"protocol wildcard denies", []SdfRule{sdfRuleIPv4(dst, 32, 0, 0, SdfProtoAny, SdfActionDeny)}, udp53, true},
		{"protocol mismatch passes", []SdfRule{sdfRuleIPv4(dst, 32, 0, 0, protoTCP, SdfActionDeny)}, udp53, false},
		{"protocol match denies (tcp)", []SdfRule{sdfRuleIPv4(dst, 32, 0, 0, protoTCP, SdfActionDeny)}, tcp80, true},
		{"port in range denies", []SdfRule{sdfRuleIPv4(dst, 32, 50, 60, protoUDP, SdfActionDeny)}, udp53, true},
		{"port out of range passes", []SdfRule{sdfRuleIPv4(dst, 32, 100, 200, protoUDP, SdfActionDeny)}, udp53, false},
		{"port wildcard denies", []SdfRule{sdfRuleIPv4(dst, 32, 0, 0, protoUDP, SdfActionDeny)}, udp53, true},
		{"cidr match denies", []SdfRule{sdfRuleIPv4([4]byte{8, 8, 0, 0}, 16, 0, 0, protoUDP, SdfActionDeny)}, udp53, true},
		{"cidr miss passes", []SdfRule{sdfRuleIPv4([4]byte{9, 9, 0, 0}, 16, 0, 0, protoUDP, SdfActionDeny)}, udp53, false},
		{"prefix wildcard denies", []SdfRule{sdfRuleIPv4([4]byte{}, 0, 0, 0, protoUDP, SdfActionDeny)}, udp53, true},
		{"first match allow wins", []SdfRule{sdfRuleIPv4(dst, 32, 0, 0, protoUDP, SdfActionAllow), sdfRuleIPv4(dst, 32, 0, 0, protoUDP, SdfActionDeny)}, udp53, false},
		{"first match deny wins", []SdfRule{sdfRuleIPv4(dst, 32, 0, 0, protoUDP, SdfActionDeny), sdfRuleIPv4(dst, 32, 0, 0, protoUDP, SdfActionAllow)}, udp53, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			putSDFFilter(t, obj, filterIndex, tc.rules)

			action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, tc.inner))

			if tc.wantDrop {
				if action != ActionDrop {
					t.Fatalf("got XDP action %d, want ActionDrop", action)
				}

				return
			}

			if action == ActionDrop {
				t.Fatal("allowed packet was dropped")
			}

			if !bytes.Equal(out[ethHdrLen:], tc.inner) {
				t.Fatalf("allowed inner packet altered:\n got %x\nwant %x", out[ethHdrLen:], tc.inner)
			}
		})
	}
}

// TestSDFDownlinkDirection checks that downlink SDF matches the remote (the
// packet source), the opposite of the uplink direction.
func TestSDFDownlinkDirection(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444602
		filterIndex = 1
		qfi         = 3
	)

	obj := loadProgram(t, 1, 0)

	ueIP := [4]byte{10, 45, 0, 2}
	server := [4]byte{8, 8, 8, 8}
	local := [4]byte{192, 168, 100, 1}
	remote := [4]byte{192, 168, 100, 9}

	putDownlinkPDRFiltered(t, obj, ueIP, teid, local, remote, qfi, filterIndex)

	inner := ipv4Packet(server, ueIP, 17, udpDatagram(4000, 4001, nil))

	t.Run("deny by source drops", func(t *testing.T) {
		putSDFFilter(t, obj, filterIndex, []SdfRule{sdfRuleIPv4(server, 32, 0, 0, 17, SdfActionDeny)})

		action, _ := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
		if action != ActionDrop {
			t.Fatalf("got XDP action %d, want ActionDrop", action)
		}
	})

	t.Run("non-matching source passes and encapsulates", func(t *testing.T) {
		putSDFFilter(t, obj, filterIndex, []SdfRule{sdfRuleIPv4([4]byte{1, 1, 1, 1}, 32, 0, 0, 17, SdfActionDeny)})

		action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
		if action == ActionDrop {
			t.Fatal("allowed downlink packet was dropped")
		}

		if f := parseGTPv4Frame(t, out); !bytes.Equal(f.inner, inner) {
			t.Fatalf("inner packet altered by encapsulation:\n got %x\nwant %x", f.inner, inner)
		}
	})
}

// TestSDFIPv6 checks IPv6 prefix matching (uplink, inner IPv6).
func TestSDFIPv6(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444603
		filterIndex = 1
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, filterIndex)

	dst := testUEv6 // inner daddr is the SDF remote on uplink
	other := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
	inner := innerIPv6UDP(dst, 53)

	tests := []struct {
		name     string
		rules    []SdfRule
		wantDrop bool
	}{
		{"/128 match denies", []SdfRule{sdfRuleIPv6(dst, 128, 0, 0, 17, SdfActionDeny)}, true},
		{"/64 match denies", []SdfRule{sdfRuleIPv6(dst, 64, 0, 0, 17, SdfActionDeny)}, true},
		{"/128 mismatch passes", []SdfRule{sdfRuleIPv6(other, 128, 0, 0, 17, SdfActionDeny)}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			putSDFFilter(t, obj, filterIndex, tc.rules)

			action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

			if tc.wantDrop {
				if action != ActionDrop {
					t.Fatalf("got XDP action %d, want ActionDrop", action)
				}

				return
			}

			if action == ActionDrop {
				t.Fatal("allowed packet was dropped")
			}

			if !bytes.Equal(out[ethHdrLen:], inner) {
				t.Fatalf("allowed inner packet altered:\n got %x\nwant %x", out[ethHdrLen:], inner)
			}
		})
	}
}

// TestSDFDownlinkIPv6 checks IPv6 downlink SDF matching on the remote (the
// packet source), the IPv6 counterpart to TestSDFDownlinkDirection.
func TestSDFDownlinkIPv6(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444604
		filterIndex = 1
		qfi         = 3
	)

	obj := loadProgram(t, 1, 0)

	uePrefix := netip.MustParseAddr("2001:db8::")
	serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
	pdr.FilterMapIndex = filterIndex

	if err := obj.PutPdrDownlink(uePrefix, pdr); err != nil {
		t.Fatalf("install downlink IPv6 PDR: %v", err)
	}

	inner := ipv6Packet(serverV6, testUEv6, 17, udpDatagram(4000, 53, nil))

	t.Run("deny by source drops", func(t *testing.T) {
		putSDFFilter(t, obj, filterIndex, []SdfRule{sdfRuleIPv6(serverV6, 128, 0, 0, 17, SdfActionDeny)})

		action, _ := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x86DD, inner))
		if action != ActionDrop {
			t.Fatalf("got XDP action %d, want ActionDrop", action)
		}
	})

	t.Run("non-matching source passes and encapsulates", func(t *testing.T) {
		other := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
		putSDFFilter(t, obj, filterIndex, []SdfRule{sdfRuleIPv6(other, 128, 0, 0, 17, SdfActionDeny)})

		action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x86DD, inner))
		if action == ActionDrop {
			t.Fatal("allowed downlink packet was dropped")
		}

		if f := parseGTPv4Frame(t, out); !bytes.Equal(f.inner, inner) {
			t.Fatalf("inner packet altered by encapsulation:\n got %x\nwant %x", f.inner, inner)
		}
	})
}

// TestSDFPortRangeStartingAtZero: only low == high == 0 is the wildcard, so a
// range starting at 0 is still a range. Gating on port_low alone drops
// port_high, and a rule for the well-known ports then matches every port.
func TestSDFPortRangeStartingAtZero(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444605
		filterIndex = 1
		protoUDP    = 17
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, filterIndex)

	dst := [4]byte{8, 8, 8, 8}

	var (
		denyWellKnown  = sdfRuleIPv4(dst, 32, 0, 1023, protoUDP, SdfActionDeny)
		allowWellKnown = sdfRuleIPv4(dst, 32, 0, 1023, protoUDP, SdfActionAllow)
		denyAnyPort    = sdfRuleIPv4(dst, 32, 0, 0, protoUDP, SdfActionDeny)
	)

	tests := []struct {
		name     string
		rules    []SdfRule
		dport    uint16
		wantDrop bool
	}{
		{"deny 0-1023 drops a port inside the range", []SdfRule{denyWellKnown}, 53, true},
		{"deny 0-1023 does not match a port above it", []SdfRule{denyWellKnown}, 8080, false},
		{"allow 0-1023 does not shadow the deny that follows it", []SdfRule{allowWellKnown, denyAnyPort}, 8080, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			putSDFFilter(t, obj, filterIndex, tc.rules)

			before := DropCount(obj, Uplink, "sdf_filter")

			inner := innerIPv4UDP(dst, tc.dport)

			action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))
			denied := DropCount(obj, Uplink, "sdf_filter") - before

			if tc.wantDrop {
				if action != ActionDrop || denied != 1 {
					t.Fatalf("port %d: got XDP action %d with %d SDF denials, want ActionDrop (%d) with 1", tc.dport, action, denied, ActionDrop)
				}

				return
			}

			if denied != 0 {
				t.Fatalf("port %d: packet denied by the SDF filter, want the rule not to match", tc.dport)
			}

			if action == ActionDrop {
				t.Fatalf("port %d: allowed packet was dropped", tc.dport)
			}

			if !bytes.Equal(out[ethHdrLen:], inner) {
				t.Fatalf("allowed inner packet altered:\n got %x\nwant %x", out[ethHdrLen:], inner)
			}
		})
	}
}

// sdfRuleIPv4 builds an SDF rule for an IPv4 remote prefix, port range, and
// protocol. A prefixLen or port bound of 0 is a wildcard in the data plane.
func sdfRuleIPv4(remote [4]byte, prefixLen uint8, portLow, portHigh uint16, proto, action uint8) SdfRule {
	return SdfRule{
		RemoteIP:  IPToIn6Addr(netip.AddrFrom4(remote)),
		PrefixLen: prefixLen,
		PortLow:   portLow,
		PortHigh:  portHigh,
		Protocol:  proto,
		Action:    action,
	}
}

// sdfRuleIPv6 builds an SDF rule for a native IPv6 remote prefix.
func sdfRuleIPv6(remote [16]byte, prefixLen uint8, portLow, portHigh uint16, proto, action uint8) SdfRule { //nolint:unparam // general-purpose builder; port bounds vary across callers
	return SdfRule{
		RemoteIP:  remote,
		PrefixLen: prefixLen,
		PortLow:   portLow,
		PortHigh:  portHigh,
		Protocol:  proto,
		Action:    action,
	}
}

func putSDFFilter(t *testing.T, obj *BpfObjects, index uint32, rules []SdfRule) { //nolint:unparam // general helper; the filter index is configurable
	t.Helper()

	var list SdfFilterList

	list.NumRules = uint8(len(rules))
	copy(list.Rules[:], rules)

	if err := obj.PutSdfFilterList(index, list); err != nil {
		t.Fatalf("install SDF filter: %v", err)
	}
}

// TestSdfFilterListRoundTrip: a slot reads back as written, and releasing it
// zeroes it — a released slot keeping its rules would enforce them for whichever
// policy is handed the index next.
func TestSdfFilterListRoundTrip(t *testing.T) {
	requireProgTestRun(t)

	const index = 7

	obj := loadN3N6Program(t)

	rules := []SdfRule{
		sdfRuleIPv4([4]byte{8, 8, 8, 8}, 32, 53, 53, 17, SdfActionDeny),
		sdfRuleIPv4([4]byte{1, 1, 1, 1}, 32, 0, 0, SdfProtoAny, SdfActionAllow),
	}

	putSDFFilter(t, obj, index, rules)

	var stored SdfFilterList
	if err := obj.SdfFilters.Lookup(uint32(index), &stored); err != nil {
		t.Fatalf("read back filter list: %v", err)
	}

	if stored.NumRules != uint8(len(rules)) {
		t.Errorf("NumRules = %d, want %d", stored.NumRules, len(rules))
	}

	if stored.Rules[0] != rules[0] || stored.Rules[1] != rules[1] {
		t.Errorf("rules altered in the round trip:\n got %+v\nwant %+v", stored.Rules[:2], rules)
	}

	if err := obj.DeleteSdfFilterList(index); err != nil {
		t.Fatalf("delete filter list: %v", err)
	}

	var cleared SdfFilterList
	if err := obj.SdfFilters.Lookup(uint32(index), &cleared); err != nil {
		t.Fatalf("read back cleared slot: %v", err)
	}

	if cleared != (SdfFilterList{}) {
		t.Errorf("released slot still holds state: %+v", cleared)
	}
}

// TestSdfFilterListLayout pins the Go mirror of struct sdf_filter_list under
// both views: writes go through unsafe.Pointer, reads through encoding/binary,
// which ignores implicit padding and fails with "doesn't consume all data".
func TestSdfFilterListLayout(t *testing.T) {
	var (
		rule SdfRule
		list SdfFilterList
	)

	const (
		ruleSize = 32                       // sizeof(struct sdf_rule)
		listSize = 4 + MaxRulesPerFilter*32 // sizeof(struct sdf_filter_list)
	)

	if got := unsafe.Sizeof(rule); got != ruleSize {
		t.Errorf("unsafe.Sizeof(SdfRule) = %d, want %d", got, ruleSize)
	}

	if got := binary.Size(rule); got != ruleSize {
		t.Errorf("binary.Size(SdfRule) = %d, want %d: some padding is implicit", got, ruleSize)
	}

	if got := unsafe.Sizeof(list); got != listSize {
		t.Errorf("unsafe.Sizeof(SdfFilterList) = %d, want %d", got, listSize)
	}

	if got := binary.Size(list); got != listSize {
		t.Errorf("binary.Size(SdfFilterList) = %d, want %d: some padding is implicit", got, listSize)
	}
}
