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

	oversize := rsMessage(0, append([]byte{200, (RSMsgMax + 64) / 8}, make([]byte, RSMsgMax+62)...))

	cases := []struct {
		name   string
		src    [16]byte
		msg    []byte
		mangle func([]byte) []byte
		msgLen int
	}{
		{"link-local source", testUELinkLocal, rsMessage(0), nil, 0},
		{"source link-layer address option", testUELinkLocal, rsMessage(0, slla()), nil, 0},
		{"unrecognized option", testUELinkLocal, rsMessage(0, []byte{200, 1, 0, 0, 0, 0, 0, 0}), nil, 0},
		{"nonzero reserved field", testUELinkLocal, []byte{133, 0, 0, 0, 0xde, 0xad, 0xbe, 0xef}, nil, 0},
		{"unspecified source without options", unspecifiedV6, rsMessage(0), nil, 0},
		{"at the size bound", testUELinkLocal, rsMessage(0, append([]byte{200, (RSMsgMax - 8) / 8}, make([]byte, RSMsgMax-10)...)), nil, 0},
		{"invalid checksum", testUELinkLocal, rsMessage(0), badChecksum, 0},
		{"option with zero length", testUELinkLocal, rsMessage(0, []byte{1, 0, 0, 0, 0, 0, 0, 0}), nil, 0},
		{"unspecified source with source link-layer address", unspecifiedV6, rsMessage(0, slla()), nil, 0},
		{"larger than the size bound", testUELinkLocal, oversize, nil, len(oversize)},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			teid := uint32(0x52540001 + i)

			obj := loadN3N6Program(t)
			putForwardingUplinkPDR(t, obj, teid, 0)

			inner := routerSolicitation(tc.src, nil, tc.msg)
			if tc.mangle != nil {
				inner = tc.mangle(inner)
			}

			ev := assertRSIntercepted(t, obj, teid, inner, tc.src)

			if tc.msgLen != 0 {
				if int(ev.MsgLen) != tc.msgLen {
					t.Fatalf("RS event message length = %d, want %d", ev.MsgLen, tc.msgLen)
				}

				return
			}

			assertRSMessage(t, ev, inner[40:])
		})
	}
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

	fragmented := func() []byte {
		pkt := routerSolicitation(testUELinkLocal, ipv6FragmentHeader(58, 0, true), rsMessage(0))
		pkt[6] = ipprotoFragment

		return pkt
	}

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
		{"first fragment", fragmented()},
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
