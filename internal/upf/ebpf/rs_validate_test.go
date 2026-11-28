// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"encoding/binary"
	"testing"

	"github.com/cilium/ebpf/ringbuf"
)

var (
	testUELinkLocal = [16]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x02}
	unspecifiedV6   = [16]byte{}
)

func rsMessage(code uint8, options ...[]byte) []byte {
	msg := []byte{133, code, 0, 0, 0, 0, 0, 0}
	for _, o := range options {
		msg = append(msg, o...)
	}

	return msg
}

func slla() []byte {
	return []byte{1, 1, 0x02, 0, 0, 0, 0, 0x01}
}

func TestRouterSolicitationPassedToUserspace(t *testing.T) {
	requireProgTestRun(t)

	badChecksum := func(pkt []byte) []byte {
		pkt[len(pkt)-1] ^= 0xff

		return pkt
	}

	cases := []struct {
		name   string
		src    [16]byte
		chain  []byte
		msg    []byte
		mangle func([]byte) []byte
	}{
		{"link-local source", testUELinkLocal, nil, rsMessage(0), nil},
		{"source link-layer address option", testUELinkLocal, nil, rsMessage(0, slla()), nil},
		{"unrecognized option", testUELinkLocal, nil, rsMessage(0, []byte{200, 1, 0, 0, 0, 0, 0, 0}), nil},
		{"nonzero reserved field", testUELinkLocal, nil, []byte{133, 0, 0, 0, 0xde, 0xad, 0xbe, 0xef}, nil},
		{"unspecified source without options", unspecifiedV6, nil, rsMessage(0), nil},
		{"at the size bound", testUELinkLocal, nil, rsAtSizeBound(), nil},
		{"at the size bound behind a Hop-by-Hop Options header", testUELinkLocal, hopByHopHeader(58), rsAtSizeBound(), nil},
		{"invalid checksum", testUELinkLocal, nil, rsMessage(0), badChecksum},
		{"option with zero length", testUELinkLocal, nil, rsMessage(0, []byte{1, 0, 0, 0, 0, 0, 0, 0}), nil},
		{"unspecified source with source link-layer address", unspecifiedV6, nil, rsMessage(0, slla()), nil},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			teid := uint32(0x52540001 + i)

			obj := loadN3N6Program(t)
			putForwardingUplinkPDR(t, obj, teid, 0)

			inner := routerSolicitation(tc.src, tc.chain, tc.msg)
			if tc.mangle != nil {
				inner = tc.mangle(inner)
			}

			ev := assertRSIntercepted(t, obj, teid, inner, tc.src)

			assertRSMessage(t, ev, inner[40+len(tc.chain):])
		})
	}
}

func rsAtSizeBound() []byte {
	return rsMessage(0, append([]byte{200, (RSMsgMax - 8) / 8}, make([]byte, RSMsgMax-10)...))
}

func atomicFragmentHeader(nextHdr uint8) []byte {
	return ipv6FragmentHeader(nextHdr, 0, false)
}

func TestRouterSolicitationInvalidDiscarded(t *testing.T) {
	requireProgTestRun(t)

	hopLimit := func(pkt []byte, v uint8) []byte {
		pkt[7] = v

		return pkt
	}

	payloadLen := func(pkt []byte, v uint16) []byte {
		binary.BigEndian.PutUint16(pkt[4:6], v)

		return pkt
	}

	fragmented := func(frag []byte) []byte {
		pkt := routerSolicitation(testUELinkLocal, frag, rsMessage(0))
		pkt[6] = ipprotoFragment

		return pkt
	}

	oversize := rsMessage(0, append([]byte{200, (RSMsgMax + 8) / 8}, make([]byte, RSMsgMax+6)...))

	cases := []struct {
		name  string
		inner []byte
	}{
		{"hop limit below 255", hopLimit(routerSolicitation(testUELinkLocal, nil, rsMessage(0)), 254)},
		{"hop limit 64", hopLimit(routerSolicitation(testUELinkLocal, nil, rsMessage(0)), 64)},
		{"nonzero code", routerSolicitation(testUELinkLocal, nil, rsMessage(1))},
		{"shorter than 8 octets", routerSolicitation(testUELinkLocal, nil, []byte{133, 0, 0, 0})},
		{"payload length short of the header", payloadLen(routerSolicitation(testUELinkLocal, nil, rsMessage(0)), 4)},
		{"payload length past the frame", payloadLen(routerSolicitation(testUELinkLocal, nil, rsMessage(0)), 64)},
		{"behind an extension header, shorter than 8 octets", routerSolicitation(testUELinkLocal, hopByHopHeader(58), []byte{133, 0, 0, 0})},
		{"first fragment", fragmented(ipv6FragmentHeader(58, 0, true))},
		{"atomic fragment", fragmented(atomicFragmentHeader(58))},
		{"one octet over the size bound", routerSolicitation(testUELinkLocal, nil, oversize[:RSMsgMax+1])},
		{"larger than the size bound", routerSolicitation(testUELinkLocal, nil, oversize)},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			teid := uint32(0x52550001 + i)

			obj := loadN3N6Program(t)
			putForwardingUplinkPDR(t, obj, teid, 0)

			rd, err := ringbuf.NewReader(obj.RsEventMap)
			if err != nil {
				t.Fatalf("open rs_event ring buffer: %v", err)
			}

			defer func() { _ = rd.Close() }()

			if reason := uplinkDropReason(t, obj, teid, tc.inner); reason != "rs_invalid" {
				t.Fatalf("drop reason = %q, want rs_invalid", reasonOrDelivered(reason))
			}

			if ev := readRSEvent(t, rd); ev != nil {
				t.Errorf("invalid Router Solicitation reached the RA responder (TEID %#x)", ev.TEID)
			}
		})
	}
}

func TestRouterSolicitationRingCapacity(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x52560001
		sent = 2048
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	msg := rsMessage(0)
	frame := uplinkGPDU(teid, routerSolicitation(testUELinkLocal, nil, msg))

	for range sent {
		if action := runXDP(t, obj.UpfEntryFunc, frame); action != ActionDrop {
			t.Fatalf("Router Solicitation not intercepted: got XDP action %d", action)
		}
	}

	rd, err := ringbuf.NewReader(obj.RsEventMap)
	if err != nil {
		t.Fatalf("open rs_event ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	buffered := 0
	for readRSEvent(t, rd) != nil {
		buffered++
	}

	lost, err := RingbufLost(obj)
	if err != nil {
		t.Fatal(err)
	}

	if uint64(buffered)+lost["rs_event_map"] != sent {
		t.Fatalf("%d Router Solicitations buffered and %d counted lost, want %d in all", buffered, lost["rs_event_map"], sent)
	}

	const ringbufHdrLen = 8

	record := (RSEventHeaderLen + len(msg) + ringbufHdrLen + 7) &^ 7
	if want := int(obj.RsEventMap.MaxEntries()) / record; buffered != want {
		t.Fatalf("ring holds %d %d-octet Router Solicitations, want %d %d-byte records", buffered, len(msg), want, record)
	}
}

func TestAtomicFragmentOutsideNeighborDiscoveryUnchanged(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x52570001
		filteredTEI = 0x52570002
		filterIndex = 1
		dport       = 53
		protoUDP    = 17
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)
	putForwardingUplinkPDR(t, obj, filteredTEI, filterIndex)

	echo := icmpv6Checksummed(testUEv6Src, testUEv6, []byte{128, 0, 0, 0, 0, 1, 0, 1})
	inner := ipv6Packet(testUEv6Src, testUEv6, ipprotoFragment, append(atomicFragmentHeader(58), echo...))

	if reason := uplinkDropReason(t, obj, teid, inner); reason != "" {
		t.Fatalf("ICMPv6 Echo Request in an atomic fragment: %s, want delivered", reasonOrDelivered(reason))
	}

	putSDFFilter(t, obj, filterIndex, []SdfRule{sdfRuleIPv6(testUEv6, 128, dport, dport, protoUDP, SdfActionDeny)})

	udp := ipv6Packet(testUEv6Src, testUEv6, ipprotoFragment, append(atomicFragmentHeader(protoUDP), udpDatagram(0, dport, nil)...))

	if reason := uplinkDropReason(t, obj, filteredTEI, udp); reason != "sdf_filter" {
		t.Fatalf("UDP in an atomic fragment under a port-scoped deny: %s, want sdf_filter", reasonOrDelivered(reason))
	}
}
