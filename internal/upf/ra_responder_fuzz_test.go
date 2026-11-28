// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/ndp"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func fuzzRSRecord(teid uint32, src, dst string, msg []byte, fixChecksum bool) []byte {
	s, d := netip.MustParseAddr(src), netip.MustParseAddr(dst)

	if fixChecksum && len(msg) >= 4 {
		msg[2], msg[3] = 0, 0
		ndp.SetICMPv6Checksum(s, d, msg)
	}

	rec := make([]byte, ebpf.RSEventHeaderLen, ebpf.RSEventHeaderLen+len(msg))
	binary.NativeEndian.PutUint32(rec[0:4], teid)

	s16, d16 := s.As16(), d.As16()
	copy(rec[4:20], s16[:])
	copy(rec[20:36], d16[:])
	binary.NativeEndian.PutUint16(rec[36:38], uint16(len(msg)))

	return append(rec, msg...)
}

func validRSOptions(src netip.Addr, opts []byte) bool {
	for len(opts) > 0 {
		if len(opts) < 2 || opts[1] == 0 || int(opts[1])*8 > len(opts) {
			return false
		}

		if opts[0] == ndp.NDPOptionSourceLinkLayerAddress && src.IsUnspecified() {
			return false
		}

		opts = opts[int(opts[1])*8:]
	}

	return true
}

func FuzzRSEvent(f *testing.F) {
	prefix := netip.MustParseAddr("2001:db8:abcd:1000::").AsSlice()
	v4 := netip.MustParseAddr("10.45.0.1").AsSlice()
	sllao := []byte{ndp.NDPOptionSourceLinkLayerAddress, 1, 0x02, 0, 0, 0, 0, 0x01}

	rs := func(options ...[]byte) []byte {
		msg := []byte{ndp.ICMPv6TypeRouterSolicitation, 0, 0, 0, 0, 0, 0, 0}
		for _, o := range options {
			msg = append(msg, o...)
		}

		return msg
	}

	valid := fuzzRSRecord(0xabcd, "fe80::1234:5678:9abc:def0", "ff02::2", rs(), true)

	f.Add(valid, prefix, 64, uint32(1500))
	f.Add(fuzzRSRecord(1, "fe80::2", "ff02::2", rs(sllao), true), prefix, 64, uint32(0))
	f.Add(fuzzRSRecord(0xffffffff, "::", "ff02::2", rs(), true), prefix, 56, uint32(1280))
	f.Add(fuzzRSRecord(2, "::", "ff02::2", rs(sllao), true), prefix, 64, uint32(1500))
	f.Add(fuzzRSRecord(3, "::", "ff02::2", rs([]byte{200, 1, 0, 0, 0, 0, 0, 0}), true), prefix, 64, uint32(1500))
	f.Add(fuzzRSRecord(4, "fe80::2", "fe80::1", rs([]byte{200, 0, 0, 0, 0, 0, 0, 0}), true), prefix, 64, uint32(1500))
	f.Add(fuzzRSRecord(5, "fe80::2", "ff02::2", rs([]byte{200, 2, 0, 0, 0, 0, 0, 0}), true), prefix, 64, uint32(1500))
	f.Add(fuzzRSRecord(6, "fe80::2", "ff02::2", rs(), false), prefix, 64, uint32(1500))
	f.Add(fuzzRSRecord(7, "fe80::2", "ff02::2", []byte{133, 1, 0, 0, 0, 0, 0, 0}, true), prefix, 64, uint32(1500))
	f.Add(fuzzRSRecord(0, "ff02::2", "ff02::2", rs(), true), prefix, 128, uint32(0xffffffff))
	f.Add(fuzzRSRecord(7, "::ffff:10.45.0.1", "ff02::2", rs(), true), prefix, 0, uint32(9000))
	f.Add(fuzzRSRecord(7, "fe80::2", "ff02::2", rs(), true), v4, 24, uint32(1500))
	f.Add(fuzzRSRecord(7, "fe80::2", "ff02::2", rs(), true), prefix, 129, uint32(1500))
	f.Add(fuzzRSRecord(7, "fe80::2", "ff02::2", rs(), true), prefix, -1, uint32(1500))
	f.Add(valid[:ebpf.RSEventHeaderLen-1], prefix, 64, uint32(1500))
	f.Add(append(append([]byte{}, valid...), 0), prefix, 64, uint32(1500))
	f.Add([]byte{}, []byte{}, 64, uint32(1500))

	f.Fuzz(func(t *testing.T, raw []byte, prefixAddr []byte, bits int, mtu uint32) {
		ev, err := ebpf.ParseRSEvent(raw)
		if err != nil {
			if len(raw) >= ebpf.RSEventHeaderLen && int(binary.NativeEndian.Uint16(raw[36:38])) == len(raw)-ebpf.RSEventHeaderLen {
				t.Fatalf("consistent %d-byte record rejected: %v", len(raw), err)
			}

			return
		}

		src := netip.AddrFrom16(ev.UEIPv6)
		dst := netip.AddrFrom16(ev.DstIPv6)

		reason := rsDiscardReason(&ev)
		if accepted := acceptRS(&ev); accepted != (reason == "") {
			t.Fatalf("acceptRS = %v with discard reason %q", accepted, reason)
		}

		if reason != "" {
			return
		}

		if len(ev.Msg) < 8 || ev.Msg[0] != ndp.ICMPv6TypeRouterSolicitation || ev.Msg[1] != 0 {
			t.Fatalf("accepted a message that is not a Router Solicitation: % x", ev.Msg)
		}

		if ndp.ICMPv6Checksum(src, dst, ev.Msg) != 0 {
			t.Fatalf("accepted a Router Solicitation whose checksum does not verify: % x", ev.Msg)
		}

		if !validRSOptions(src, ev.Msg[8:]) {
			t.Fatalf("accepted a Router Solicitation from %s with invalid options: % x", src, ev.Msg[8:])
		}

		addr, _ := netip.AddrFromSlice(prefixAddr)
		sessionPrefix := netip.PrefixFrom(addr, bits)

		r := &RAResponder{sessions: make(map[uint32]*IPv6SessionContext), injectFD: -1}
		r.RegisterSession(ev.TEID, &IPv6SessionContext{Prefix: sessionPrefix, MTU: mtu})

		pkt, err := r.solicitedRA(ev.TEID, src)
		if err != nil {
			if sessionPrefix.IsValid() {
				t.Fatalf("no RA for valid prefix %s: %v", sessionPrefix, err)
			}

			return
		}

		if !sessionPrefix.IsValid() {
			t.Fatal("built an RA for an invalid prefix")
		}

		ra := parseInjectedRA(t, pkt)

		if want := netip.AddrFrom16(sessionPrefix.Masked().Addr().As16()); ra.session != want {
			t.Fatalf("RA carried into session %s, want the soliciting session %s", ra.session, want)
		}

		wantDst := src
		if src.IsUnspecified() {
			wantDst = ndp.AllNodesMulticast
		}

		if ra.src != routerLinkLocal || ra.dst != wantDst {
			t.Fatalf("RA for an RS from %s addressed %s -> %s, want %s -> %s", src, ra.src, ra.dst, routerLinkLocal, wantDst)
		}

		want := 16 + 32
		if mtu > 0 {
			want += 8
		}

		if len(ra.body) != want {
			t.Fatalf("RA body is %d bytes, want %d", len(ra.body), want)
		}
	})
}
