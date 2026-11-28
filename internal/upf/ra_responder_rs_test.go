// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/ndp"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	dto "github.com/prometheus/client_model/go"
	"golang.org/x/sys/unix"
)

var (
	rsTestSrc = netip.MustParseAddr("fe80::2")
	rsTestDst = netip.MustParseAddr("ff02::2")
)

func rsEvent(src, dst netip.Addr, msg []byte, fixChecksum bool) *ebpf.RSEvent {
	if fixChecksum && len(msg) >= 4 {
		msg[2], msg[3] = 0, 0
		ndp.SetICMPv6Checksum(src, dst, msg)
	}

	return &ebpf.RSEvent{
		TEID:    0x1234,
		UEIPv6:  src.As16(),
		DstIPv6: dst.As16(),
		Msg:     msg,
	}
}

func rsBody(options ...[]byte) []byte {
	msg := []byte{ndp.ICMPv6TypeRouterSolicitation, 0, 0, 0, 0, 0, 0, 0}
	for _, o := range options {
		msg = append(msg, o...)
	}

	return msg
}

func discardCount(t *testing.T, reason string) float64 {
	t.Helper()

	var m dto.Metric
	if err := routerSolicitationsDiscarded.WithLabelValues(reason).Write(&m); err != nil {
		t.Fatalf("read %s counter: %v", reason, err)
	}

	return m.GetCounter().GetValue()
}

func TestRouterSolicitationDiscardReason(t *testing.T) {
	unspecified := netip.IPv6Unspecified()
	slla := []byte{ndp.NDPOptionSourceLinkLayerAddress, 1, 0x02, 0, 0, 0, 0, 0x01}

	cases := []struct {
		name string
		ev   *ebpf.RSEvent
		want string
	}{
		{"valid", rsEvent(rsTestSrc, rsTestDst, rsBody(), true), ""},
		{"valid with source link-layer address", rsEvent(rsTestSrc, rsTestDst, rsBody(slla), true), ""},
		{"valid with unrecognized option", rsEvent(rsTestSrc, rsTestDst, rsBody([]byte{200, 1, 0, 0, 0, 0, 0, 0}), true), ""},
		{"valid with nonzero reserved field", rsEvent(rsTestSrc, rsTestDst, []byte{133, 0, 0, 0, 0xde, 0xad, 0xbe, 0xef}, true), ""},
		{"valid from the unspecified address", rsEvent(unspecified, rsTestDst, rsBody(), true), ""},
		{"valid at the size bound", rsEvent(rsTestSrc, rsTestDst, rsBody(append([]byte{200, (ebpf.RSMsgMax - 8) / 8}, make([]byte, ebpf.RSMsgMax-10)...)), true), ""},
		{"invalid checksum", rsEvent(rsTestSrc, rsTestDst, rsBody(), false), rsDiscardChecksum},
		{"nonzero code", rsEvent(rsTestSrc, rsTestDst, []byte{133, 1, 0, 0, 0, 0, 0, 0}, true), rsDiscardMalformed},
		{"shorter than 8 octets", rsEvent(rsTestSrc, rsTestDst, []byte{133, 0, 0, 0}, true), rsDiscardMalformed},
		{"option with zero length", rsEvent(rsTestSrc, rsTestDst, rsBody([]byte{200, 0, 0, 0, 0, 0, 0, 0}), true), rsDiscardOptionLength},
		{"option past the end of the message", rsEvent(rsTestSrc, rsTestDst, rsBody([]byte{200, 2, 0, 0, 0, 0, 0, 0}), true), rsDiscardOptionLength},
		{"unspecified source with source link-layer address", rsEvent(unspecified, rsTestDst, rsBody(slla), true), rsDiscardUnspecifiedLLSrc},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rsDiscardReason(tc.ev); got != tc.want {
				t.Fatalf("rsDiscardReason = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAcceptRSCountsDiscards(t *testing.T) {
	slla := []byte{ndp.NDPOptionSourceLinkLayerAddress, 1, 0x02, 0, 0, 0, 0, 0x01}

	cases := []struct {
		name   string
		ev     *ebpf.RSEvent
		reason string
		accept bool
	}{
		{"valid", rsEvent(rsTestSrc, rsTestDst, rsBody(), true), rsDiscardChecksum, true},
		{"invalid checksum", rsEvent(rsTestSrc, rsTestDst, rsBody(), false), rsDiscardChecksum, false},
		{"unspecified source with source link-layer address", rsEvent(netip.IPv6Unspecified(), rsTestDst, rsBody(slla), true), rsDiscardUnspecifiedLLSrc, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := discardCount(t, tc.reason)

			if got := acceptRS(tc.ev); got != tc.accept {
				t.Fatalf("acceptRS = %v, want %v", got, tc.accept)
			}

			want := 1.0
			if tc.accept {
				want = 0
			}

			if got := discardCount(t, tc.reason) - before; got != want {
				t.Fatalf("%s discards grew by %v, want %v", tc.reason, got, want)
			}
		})
	}
}

func raSession(t *testing.T, r *RAResponder, ulTEID uint32, prefix string) netip.Prefix {
	t.Helper()

	p := netip.MustParsePrefix(prefix)
	r.RegisterSession(ulTEID, &IPv6SessionContext{Prefix: p, MTU: 1400})

	return p
}

type injectedRA struct {
	session netip.Addr
	src     netip.Addr
	dst     netip.Addr
	body    []byte
}

func parseInjectedRA(t *testing.T, pkt []byte) injectedRA {
	t.Helper()

	if len(pkt) < 14+40+40 || binary.BigEndian.Uint16(pkt[12:14]) != 0x86DD {
		t.Fatalf("not an Ethernet IPv6 frame: % x", pkt)
	}

	outer := pkt[14:]
	if outer[0]>>4 != 6 || outer[6] != unix.IPPROTO_IPV6 || int(binary.BigEndian.Uint16(outer[4:6])) != len(outer)-40 {
		t.Fatalf("bad session header % x", outer[:40])
	}

	inner := outer[40:]
	if inner[0]>>4 != 6 || inner[6] != ndp.ICMPv6ProtocolNumber || inner[7] != 255 || int(binary.BigEndian.Uint16(inner[4:6])) != len(inner)-40 {
		t.Fatalf("bad RA IPv6 header % x", inner[:40])
	}

	ra := injectedRA{
		session: netip.AddrFrom16([16]byte(outer[24:40])),
		src:     netip.AddrFrom16([16]byte(inner[8:24])),
		dst:     netip.AddrFrom16([16]byte(inner[24:40])),
		body:    inner[40:],
	}

	if ra.body[0] != ndp.ICMPv6TypeRouterAdvertisement || ndp.ICMPv6Checksum(ra.src, ra.dst, ra.body) != 0 {
		t.Fatalf("not a checksummed Router Advertisement: % x", ra.body)
	}

	return ra
}

// RFC 4861 §6.2.6
func TestSolicitedRAFromTheUnspecifiedAddressIsMulticastOnTheSolicitingSession(t *testing.T) {
	r := &RAResponder{sessions: make(map[uint32]*IPv6SessionContext), injectFD: -1}

	sessions := map[uint32]netip.Prefix{
		0x1001: raSession(t, r, 0x1001, "2001:db8:1::/64"),
		0x1002: raSession(t, r, 0x1002, "2001:db8:2::/64"),
	}

	for teid, prefix := range sessions {
		pkt, err := r.solicitedRA(teid, netip.IPv6Unspecified())
		if err != nil {
			t.Fatal(err)
		}

		ra := parseInjectedRA(t, pkt)

		if ra.dst != ndp.AllNodesMulticast {
			t.Errorf("RA for an RS from :: on TEID %#x sent to %s, want %s", teid, ra.dst, ndp.AllNodesMulticast)
		}

		if ra.session != prefix.Addr() {
			t.Errorf("RA for an RS from :: on TEID %#x carried into session %s, want the soliciting session %s", teid, ra.session, prefix.Addr())
		}
	}
}

func TestSolicitedRAIsUnicastOnTheSolicitingSession(t *testing.T) {
	r := &RAResponder{sessions: make(map[uint32]*IPv6SessionContext), injectFD: -1}

	own := raSession(t, r, 0x2001, "2001:db8:1::/64")
	raSession(t, r, 0x2002, "2001:db8:2::/64")

	for _, src := range []netip.Addr{rsTestSrc, netip.MustParseAddr("fe80::bbbb")} {
		pkt, err := r.solicitedRA(0x2001, src)
		if err != nil {
			t.Fatal(err)
		}

		ra := parseInjectedRA(t, pkt)

		if ra.dst != src || ra.src != routerLinkLocal {
			t.Errorf("RA for an RS from %s addressed %s -> %s, want %s -> %s", src, ra.src, ra.dst, routerLinkLocal, src)
		}

		if ra.session != own.Addr() {
			t.Errorf("RA for an RS from %s carried into session %s, want the soliciting session %s", src, ra.session, own.Addr())
		}
	}
}

func TestSolicitedRAForAnUnknownSession(t *testing.T) {
	r := &RAResponder{sessions: make(map[uint32]*IPv6SessionContext), injectFD: -1}

	raSession(t, r, 0x3001, "2001:db8:1::/64")
	r.UnregisterSession(0x3001)

	if _, err := r.solicitedRA(0x3001, rsTestSrc); err == nil {
		t.Fatal("built an RA for a released session")
	}
}
