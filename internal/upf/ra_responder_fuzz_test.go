// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/ndp"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func fuzzRSEventBytes(teid uint32, ue string) []byte {
	ev := ebpf.RSEvent{TEID: teid, UEIPv6: netip.MustParseAddr(ue).As16()}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.NativeEndian, ev); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

func FuzzRSEvent(f *testing.F) {
	prefix := netip.MustParseAddr("2001:db8:abcd:1000::").AsSlice()
	v4 := netip.MustParseAddr("10.45.0.1").AsSlice()

	f.Add(fuzzRSEventBytes(0xabcd, "fe80::1234:5678:9abc:def0"), prefix, 64, uint32(1500))
	f.Add(fuzzRSEventBytes(1, "fe80::2"), prefix, 64, uint32(0))
	f.Add(fuzzRSEventBytes(0xffffffff, "::"), prefix, 56, uint32(1280))
	f.Add(fuzzRSEventBytes(0, "ff02::2"), prefix, 128, uint32(0xffffffff))
	f.Add(fuzzRSEventBytes(7, "::ffff:10.45.0.1"), prefix, 0, uint32(9000))
	f.Add(fuzzRSEventBytes(7, "fe80::2"), v4, 24, uint32(1500))
	f.Add(fuzzRSEventBytes(7, "fe80::2"), prefix, 129, uint32(1500))
	f.Add(fuzzRSEventBytes(7, "fe80::2"), prefix, -1, uint32(1500))
	f.Add(fuzzRSEventBytes(7, "fe80::2")[:19], prefix, 64, uint32(1500))
	f.Add([]byte{}, []byte{}, 64, uint32(1500))

	f.Fuzz(func(t *testing.T, raw []byte, prefixAddr []byte, bits int, mtu uint32) {
		var event ebpf.RSEvent
		if err := binary.Read(bytes.NewReader(raw), binary.NativeEndian, &event); err != nil {
			if len(raw) >= binary.Size(event) {
				t.Fatalf("decode %d bytes: %v", len(raw), err)
			}

			return
		}

		addr, _ := netip.AddrFromSlice(prefixAddr)
		sess := &IPv6SessionContext{Prefix: netip.PrefixFrom(addr, bits), MTU: mtu}
		ueIPv6 := netip.AddrFrom16(event.UEIPv6)

		pkt, err := (&RAResponder{}).buildRAPacket(sess, ueIPv6)
		if err != nil {
			if sess.Prefix.IsValid() {
				t.Fatalf("buildRAPacket rejected valid prefix %s: %v", sess.Prefix, err)
			}

			return
		}

		if !sess.Prefix.IsValid() {
			t.Fatal("buildRAPacket accepted an invalid prefix")
		}

		if len(pkt) < 14+40+16 || binary.BigEndian.Uint16(pkt[12:14]) != 0x86DD {
			t.Fatalf("not an Ethernet IPv6 frame: % x", pkt)
		}

		ip := pkt[14:]
		body := ip[40:]

		if ip[0]>>4 != 6 || ip[6] != ndp.ICMPv6ProtocolNumber || ip[7] != 255 || int(binary.BigEndian.Uint16(ip[4:6])) != len(body) {
			t.Fatalf("bad IPv6 header % x", ip[:40])
		}

		src := netip.AddrFrom16([16]byte(ip[8:24]))
		dst := netip.AddrFrom16([16]byte(ip[24:40]))

		if src != routerLinkLocal || dst != ueIPv6 {
			t.Fatalf("RA addressed %s -> %s", src, dst)
		}

		if body[0] != ndp.ICMPv6TypeRouterAdvertisement || body[1] != 0 {
			t.Fatalf("ICMPv6 type/code = %d/%d", body[0], body[1])
		}

		if ndp.ICMPv6Checksum(src, dst, body) != 0 {
			t.Fatal("RA checksum does not verify")
		}

		want := 16 + 32
		if mtu > 0 {
			want += 8
		}

		if len(body) != want {
			t.Fatalf("RA body is %d bytes, want %d", len(body), want)
		}
	})
}
