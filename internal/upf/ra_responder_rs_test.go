// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/ndp"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	dto "github.com/prometheus/client_model/go"
)

var (
	rsTestSrc = netip.MustParseAddr("fe80::2")
	rsTestDst = netip.MustParseAddr("ff02::2")
)

func rsEvent(src, dst netip.Addr, msg []byte, fixChecksum bool) *ebpf.RSEvent {
	ev := &ebpf.RSEvent{
		TEID:    0x1234,
		UEIPv6:  src.As16(),
		DstIPv6: dst.As16(),
		MsgLen:  uint16(len(msg)),
	}

	if fixChecksum && len(msg) >= 4 {
		msg[2], msg[3] = 0, 0
		ndp.SetICMPv6Checksum(src, dst, msg)
	}

	copy(ev.Msg[:], msg)

	return ev
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

	oversize := rsEvent(rsTestSrc, rsTestDst, rsBody(), true)
	oversize.MsgLen = ebpf.RSMsgMax + 8

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
		{"larger than the size bound", oversize, rsDiscardTooLong},
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
