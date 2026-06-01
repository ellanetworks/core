// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
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

func putSDFFilter(t *testing.T, obj *BpfObjects, index uint32, rules []SdfRule) {
	t.Helper()

	var list SdfFilterList

	list.NumRules = uint8(len(rules))
	copy(list.Rules[:], rules)

	if err := obj.PutSdfFilterList(index, list); err != nil {
		t.Fatalf("install SDF filter: %v", err)
	}
}

// innerIPv4UDP builds the decapsulated inner packet: an IPv4/UDP datagram to
// dst:dport. On uplink, dst is the SDF remote address.
func innerIPv4UDP(dst [4]byte, dport uint16) []byte {
	const ipLen, udpLen = 20, 8

	pkt := make([]byte, ipLen+udpLen)

	ip := pkt[:ipLen]
	ip[0] = 0x45 // version 4, IHL 5
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipLen+udpLen))
	ip[8] = 64 // TTL
	ip[9] = 17 // IPPROTO_UDP
	copy(ip[12:16], []byte{10, 0, 0, 9})
	copy(ip[16:20], dst[:])

	udp := pkt[ipLen:]
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], udpLen)

	return pkt
}

// uplinkGPDU wraps inner in a well-formed GTP-U G-PDU (8-byte base header with
// the E flag set plus the 8-byte optional header word) inside an
// Ethernet/IPv4/UDP frame addressed to the GTP-U port.
func uplinkGPDU(teid uint32, inner []byte) []byte {
	const gtpLen = 16 // base header + optional header word

	gtp := make([]byte, gtpLen)
	gtp[0] = 0x34 // version=1, PT=1, E=1
	gtp[1] = 0xFF // GTPU_G_PDU
	binary.BigEndian.PutUint16(gtp[2:4], uint16(gtpLen-8+len(inner)))
	binary.BigEndian.PutUint32(gtp[4:8], teid)

	return wrapIPv4UDP(append(gtp, inner...), GTPUDPPort)
}
