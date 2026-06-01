// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"net/netip"
	"testing"
)

// TestSDFFilterEnforcement checks that uplink SDF rules drop denied traffic and
// pass everything else.
//
// A deny returns XDP_DROP before routing, so a denied packet is unambiguously
// XDP_DROP. An allowed packet is decapsulated and continues into the routing
// tail; it must not be XDP_DROP (routing returns XDP_TX/XDP_REDIRECT, or
// XDP_PASS with no route, but not XDP_DROP absent blackhole/unreachable routes)
// and its decapsulated inner packet must be intact. The verdict and output
// packet come from the program; BPF_PROG_TEST_RUN does not surface counters.
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

			action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(tc.teid, inner))

			if tc.wantDrop {
				if action != XDP_DROP {
					t.Fatalf("denied packet: got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
				}

				return
			}

			if action == XDP_DROP {
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
// wildcard), and first-match ordering. Deny => XDP_DROP; allow => forwarded with
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

			action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, tc.inner))

			if tc.wantDrop {
				if action != XDP_DROP {
					t.Fatalf("got XDP action %d, want XDP_DROP", action)
				}

				return
			}

			if action == XDP_DROP {
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

		action, _ := runXDPOut(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))
		if action != XDP_DROP {
			t.Fatalf("got XDP action %d, want XDP_DROP", action)
		}
	})

	t.Run("non-matching source passes and encapsulates", func(t *testing.T) {
		putSDFFilter(t, obj, filterIndex, []SdfRule{sdfRuleIPv4([4]byte{1, 1, 1, 1}, 32, 0, 0, 17, SdfActionDeny)})

		action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))
		if action == XDP_DROP {
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

			action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, inner))

			if tc.wantDrop {
				if action != XDP_DROP {
					t.Fatalf("got XDP action %d, want XDP_DROP", action)
				}

				return
			}

			if action == XDP_DROP {
				t.Fatal("allowed packet was dropped")
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
func sdfRuleIPv6(remote [16]byte, prefixLen uint8, portLow, portHigh uint16, proto, action uint8) SdfRule {
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
