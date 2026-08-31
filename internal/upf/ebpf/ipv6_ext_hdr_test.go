// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"net/netip"
	"testing"
)

func TestSDFIPv6ExtensionHeaderUplink(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444606
		filterIndex = 1
		dport       = 53
		protoUDP    = 17
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, filterIndex)

	remote := testUEv6

	tests := []struct {
		name     string
		rules    []SdfRule
		wantDrop bool
	}{
		{
			name:     "address-only deny",
			rules:    []SdfRule{sdfRuleIPv6(remote, 128, 0, 0, SdfProtoAny, SdfActionDeny)},
			wantDrop: true,
		},
		{
			name:     "protocol-scoped deny",
			rules:    []SdfRule{sdfRuleIPv6(remote, 128, 0, 0, protoUDP, SdfActionDeny)},
			wantDrop: true,
		},
		{
			name:     "port-scoped deny",
			rules:    []SdfRule{sdfRuleIPv6(remote, 128, dport, dport, SdfProtoAny, SdfActionDeny)},
			wantDrop: true,
		},
		{
			name: "allow-list ending in a catch-all deny",
			rules: []SdfRule{
				sdfRuleIPv6(remote, 128, dport, dport, protoUDP, SdfActionAllow),
				sdfRuleIPv6([16]byte{}, 0, 0, 0, SdfProtoAny, SdfActionDeny),
			},
			wantDrop: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			putSDFFilter(t, obj, filterIndex, tc.rules)

			plain := innerIPv6UDP(remote, dport)
			extended := innerIPv6UDPHopByHop(remote, dport)

			plainDrop := uplinkSDFDenied(t, obj, teid, plain)
			extendedDrop := uplinkSDFDenied(t, obj, teid, extended)

			if plainDrop != tc.wantDrop {
				t.Fatalf("plain IPv6 packet: SDF denied = %v, want %v (the rule set does not do what the test assumes)", plainDrop, tc.wantDrop)
			}

			if extendedDrop != tc.wantDrop {
				t.Errorf("IPv6 packet with a Hop-by-Hop Options header: SDF denied = %v, want %v — the same rule set denies the plain packet %v",
					extendedDrop, tc.wantDrop, plainDrop)
			}
		})
	}
}

func TestIPv6ExtensionHeaderDownlinkDelivery(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x53444607
		qfi  = 3
	)

	obj := loadProgram(t, 1, 0)

	uePrefix := netip.MustParseAddr("2001:db8::")
	serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	if err := obj.PutPdrDownlink(uePrefix, ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)); err != nil {
		t.Fatalf("install downlink IPv6 PDR: %v", err)
	}

	udp := udpDatagram(4000, 53, nil)

	tests := []struct {
		name  string
		inner []byte
	}{
		{"plain UDP", ipv6Packet(serverV6, testUEv6, 17, udp)},
		{"Hop-by-Hop Options header", ipv6Packet(serverV6, testUEv6, ipprotoHopOpts, append(hopByHopHeader(17), udp...))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x86DD, tc.inner))

			if action == ActionDrop {
				t.Fatalf("downlink packet was dropped")
			}

			if len(out) != ethHdrLen+gtpV4EncapLen+len(tc.inner) {
				t.Fatalf("downlink packet was not encapsulated: output is %d bytes, want %d (eth + GTP-U/UDP/IPv4 + inner); it was passed to the host stack instead of the session",
					len(out), ethHdrLen+gtpV4EncapLen+len(tc.inner))
			}

			if f := parseGTPv4Frame(t, out); !bytes.Equal(f.inner, tc.inner) {
				t.Fatalf("inner packet altered by encapsulation:\n got %x\nwant %x", f.inner, tc.inner)
			}
		})
	}
}

// RFC 1858
func TestSDFIPv6FragmentPortScopedPolicy(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444608
		filterIndex = 1
		dport       = 53
		protoUDP    = 17
		protoTCP    = 6
	)

	otherRemote := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x77}

	obj := loadN3N6Program(t)

	remote := testUEv6
	fragment := innerIPv6NonFirstFragment(remote, dport)

	tests := []struct {
		name        string
		filterIndex uint32
		rules       []SdfRule
		wantReason  string
	}{
		{
			name:        "port-scoped deny drops the fragment",
			filterIndex: filterIndex,
			rules:       []SdfRule{sdfRuleIPv6(remote, 128, dport, dport, protoUDP, SdfActionDeny)},
			wantReason:  "fragment_unfilterable",
		},
		{
			name:        "port-scoped allow drops the fragment too",
			filterIndex: filterIndex,
			rules:       []SdfRule{sdfRuleIPv6(remote, 128, 1, 1023, protoUDP, SdfActionAllow)},
			wantReason:  "fragment_unfilterable",
		},
		{
			name:        "port-scoped rule for another protocol is skipped",
			filterIndex: filterIndex,
			rules: []SdfRule{
				sdfRuleIPv6(remote, 128, 25, 25, protoTCP, SdfActionDeny),
				sdfRuleIPv6(remote, 128, 0, 0, SdfProtoAny, SdfActionAllow),
			},
			wantReason: "",
		},
		{
			name:        "port-scoped rule for another address is skipped",
			filterIndex: filterIndex,
			rules: []SdfRule{
				sdfRuleIPv6(otherRemote, 128, dport, dport, protoUDP, SdfActionDeny),
				sdfRuleIPv6(remote, 128, 0, 0, SdfProtoAny, SdfActionAllow),
			},
			wantReason: "",
		},
		{
			name:        "full-range port rule is evaluated normally",
			filterIndex: filterIndex,
			rules:       []SdfRule{sdfRuleIPv6(remote, 128, 0, 65535, protoUDP, SdfActionDeny)},
			wantReason:  "sdf_filter",
		},
		{
			name:        "protocol-scoped deny is evaluated normally",
			filterIndex: filterIndex,
			rules:       []SdfRule{sdfRuleIPv6(remote, 128, 0, 0, protoUDP, SdfActionDeny)},
			wantReason:  "sdf_filter",
		},
		{
			name:        "address-only allow delivers the fragment",
			filterIndex: filterIndex,
			rules:       []SdfRule{sdfRuleIPv6(remote, 128, 0, 0, SdfProtoAny, SdfActionAllow)},
			wantReason:  "",
		},
		{
			name:        "a session with no policy delivers the fragment",
			filterIndex: 0,
			wantReason:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			putForwardingUplinkPDR(t, obj, teid, tc.filterIndex)

			if tc.filterIndex != 0 {
				putSDFFilter(t, obj, tc.filterIndex, tc.rules)
			}

			if got := uplinkDropReason(t, obj, teid, fragment); got != tc.wantReason {
				t.Errorf("non-first fragment: drop reason %q, want %q", reasonOrDelivered(got), reasonOrDelivered(tc.wantReason))
			}
		})
	}
}

func TestSDFIPv6FragmentDoesNotResolveProtocolFromPayload(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444611
		filterIndex = 1
		protoUDP    = 17
		protoTCP    = 6
	)

	obj := loadN3N6Program(t)

	remote := testUEv6

	putSDFFilter(t, obj, filterIndex, []SdfRule{
		sdfRuleIPv6(remote, 128, 0, 0, protoUDP, SdfActionDeny),
	})
	putForwardingUplinkPDR(t, obj, teid, filterIndex)

	inner := innerIPv6FragmentChainingToExtHeader(remote, protoTCP)

	if reason := uplinkDropReason(t, obj, teid, inner); reason != "exthdr_invalid" {
		t.Errorf("fragment chaining into payload got drop reason %q, want exthdr_invalid", reason)
	}
}

func TestSDFIPv6FragmentKeepsProtocolFromFragmentHeader(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444612
		filterIndex = 1
		protoUDP    = 17
		dport       = 53
	)

	obj := loadN3N6Program(t)

	remote := testUEv6

	putSDFFilter(t, obj, filterIndex, []SdfRule{
		sdfRuleIPv6(remote, 128, 0, 0, protoUDP, SdfActionDeny),
	})
	putForwardingUplinkPDR(t, obj, teid, filterIndex)

	inner := innerIPv6NonFirstFragment(remote, dport)

	if reason := uplinkDropReason(t, obj, teid, inner); reason != "sdf_filter" {
		t.Errorf("later UDP fragment got drop reason %q, want sdf_filter", reason)
	}
}

func TestSDFIPv4FragmentPortScopedPolicy(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x5344460C
		filterIndex = 1
		dport       = 53
		protoUDP    = 17
		decoyPort   = 8080
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, filterIndex)

	dst := [4]byte{8, 8, 8, 8}

	tests := []struct {
		name       string
		inner      []byte
		rules      []SdfRule
		wantReason string
	}{
		{
			name:       "later fragment drops under a port-scoped deny",
			inner:      innerIPv4Fragment(dst, 1, false, decoyPort),
			rules:      []SdfRule{sdfRuleIPv4(dst, 32, dport, dport, protoUDP, SdfActionDeny)},
			wantReason: "fragment_unfilterable",
		},
		{
			name:       "first fragment is matched normally",
			inner:      innerIPv4Fragment(dst, 0, true, dport),
			rules:      []SdfRule{sdfRuleIPv4(dst, 32, dport, dport, protoUDP, SdfActionDeny)},
			wantReason: "sdf_filter",
		},
		{
			name:       "later fragment is delivered under an address-only policy",
			inner:      innerIPv4Fragment(dst, 1, false, decoyPort),
			rules:      []SdfRule{sdfRuleIPv4(dst, 32, 0, 0, SdfProtoAny, SdfActionAllow)},
			wantReason: "",
		},
		{
			name:       "protocol-scoped deny is evaluated against a later fragment",
			inner:      innerIPv4Fragment(dst, 1, false, decoyPort),
			rules:      []SdfRule{sdfRuleIPv4(dst, 32, 0, 0, protoUDP, SdfActionDeny)},
			wantReason: "sdf_filter",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			putSDFFilter(t, obj, filterIndex, tc.rules)

			if got := uplinkDropReason(t, obj, teid, tc.inner); got != tc.wantReason {
				t.Errorf("drop reason %q, want %q", reasonOrDelivered(got), reasonOrDelivered(tc.wantReason))
			}
		})
	}
}

func TestIPv6ExtensionChainTooLong(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x53444609
		filterIndex = 1
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, filterIndex)
	putSDFFilter(t, obj, filterIndex, []SdfRule{sdfRuleIPv6(testUEv6, 128, 0, 0, 17, SdfActionAllow)})

	if got := uplinkDropReason(t, obj, teid, innerIPv6ChainTooLong(testUEv6, 53)); got != "exthdr_invalid" {
		t.Errorf("over-long extension-header chain: drop reason %q, want %q", reasonOrDelivered(got), "exthdr_invalid")
	}
}

// RFC 4302 §2
func TestSDFIPv6AuthHeaderWalk(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x5344460A
		filterIndex = 1
		dport       = 53
		protoUDP    = 17
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, filterIndex)
	putSDFFilter(t, obj, filterIndex, []SdfRule{sdfRuleIPv6(testUEv6, 128, dport, dport, protoUDP, SdfActionDeny)})

	inner := ipv6Packet(testUEv6Src, testUEv6, ipprotoAH,
		append(authHeader(17), udpDatagram(0, dport, nil)...))

	if got := uplinkDropReason(t, obj, teid, inner); got != "sdf_filter" {
		t.Errorf("UDP behind an Authentication Header: drop reason %q, want the port-scoped deny to match", reasonOrDelivered(got))
	}
}

func TestDownlinkDeliversNonL4Protocols(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x5344460B
		qfi  = 3
	)

	tests := []struct {
		name  string
		proto uint8
	}{
		{"ESP", ipprotoESP},
		{"GRE", 47},
		{"SCTP", 132},
	}

	payload := make([]byte, 16)

	for _, tc := range tests {
		t.Run(tc.name+" over IPv6", func(t *testing.T) {
			obj := loadProgram(t, 1, 0)

			if err := obj.PutPdrDownlink(netip.MustParseAddr("2001:db8::"),
				ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)); err != nil {
				t.Fatalf("install downlink IPv6 PDR: %v", err)
			}

			serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}
			inner := ipv6Packet(serverV6, testUEv6, tc.proto, payload)

			requireEncapsulated(t, obj, ethFrame(0x86DD, inner), inner)
		})

		t.Run(tc.name+" over IPv4", func(t *testing.T) {
			obj := loadProgram(t, 1, 0)

			ueIP := [4]byte{10, 45, 0, 2}
			putDownlinkPDR(t, obj, ueIP, teid, [4]byte{192, 168, 100, 1}, [4]byte{192, 168, 100, 9}, qfi)

			inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, tc.proto, payload)

			requireEncapsulated(t, obj, ethFrame(0x0800, inner), inner)
		})
	}
}

func requireEncapsulated(t *testing.T, obj *BpfObjects, frame, inner []byte) {
	t.Helper()

	action, out := runXDPOut(t, obj.UpfEntryFunc, frame)
	if action == ActionDrop {
		t.Fatalf("downlink packet was dropped")
	}

	if len(out) != ethHdrLen+gtpV4EncapLen+len(inner) {
		t.Fatalf("downlink packet was not encapsulated: output is %d bytes, want %d; it was passed to the host stack instead of the session",
			len(out), ethHdrLen+gtpV4EncapLen+len(inner))
	}

	if f := parseGTPv4Frame(t, out); !bytes.Equal(f.inner, inner) {
		t.Fatalf("inner packet altered by encapsulation:\n got %x\nwant %x", f.inner, inner)
	}
}

func uplinkSDFDenied(t *testing.T, obj *BpfObjects, teid uint32, inner []byte) bool {
	t.Helper()

	switch reason := uplinkDropReason(t, obj, teid, inner); reason {
	case "":
		return false
	case "sdf_filter":
		return true
	default:
		t.Fatalf("packet dropped as %q, want an SDF verdict", reason)

		return false
	}
}

func uplinkDropReason(t *testing.T, obj *BpfObjects, teid uint32, inner []byte) string {
	t.Helper()

	before := dropCounters(obj)

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	var reason string

	for name, count := range dropCounters(obj) {
		if count > before[name] {
			if reason != "" {
				t.Fatalf("one frame was counted under two reasons: %q and %q", reason, name)
			}

			reason = name
		}
	}

	if (action == ActionDrop) != (reason != "") {
		t.Fatalf("XDP action %d with drop reason %q: a drop is counted exactly once, a forward not at all", action, reason)
	}

	if reason == "" && !bytes.Equal(out[ethHdrLen:], inner) {
		t.Fatalf("forwarded inner packet altered:\n got %x\nwant %x", out[ethHdrLen:], inner)
	}

	return reason
}

func dropCounters(obj *BpfObjects) map[string]uint64 {
	counts := make(map[string]uint64, len(DropReasonNames()))
	for _, name := range DropReasonNames() {
		counts[name] = DropCount(obj, Uplink, name)
	}

	return counts
}

func reasonOrDelivered(reason string) string {
	if reason == "" {
		return "delivered"
	}

	return reason
}
