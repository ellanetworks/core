// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ndp_test

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/ndp"
)

var (
	rsSrc        = netip.MustParseAddr("fe80::2")
	rsAllRouters = netip.MustParseAddr("ff02::2")
)

func rs(src, dst netip.Addr, code uint8, reserved [4]byte, options ...[]byte) []byte {
	msg := []byte{ndp.ICMPv6TypeRouterSolicitation, code, 0, 0, reserved[0], reserved[1], reserved[2], reserved[3]}
	for _, o := range options {
		msg = append(msg, o...)
	}

	ndp.SetICMPv6Checksum(src, dst, msg)

	return msg
}

func sllao() []byte {
	return []byte{ndp.NDPOptionSourceLinkLayerAddress, 1, 0x02, 0, 0, 0, 0, 0x01}
}

func TestValidateRouterSolicitation(t *testing.T) {
	unspecified := netip.IPv6Unspecified()

	flipLast := func(msg []byte) []byte {
		msg[len(msg)-1] ^= 0xff

		return msg
	}

	cases := []struct {
		name string
		src  netip.Addr
		dst  netip.Addr
		msg  []byte
		want error
	}{
		{"no options", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}), nil},
		{"source link-layer address option", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}, sllao()), nil},
		{"unrecognized option", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}, []byte{200, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}), nil},
		{"option not used with RS", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}, []byte{ndp.NDPOptionMTU, 1, 0, 0, 0, 0, 5, 0xdc}), nil},
		{"nonzero reserved field", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{0xde, 0xad, 0xbe, 0xef}), nil},
		{"unicast destination", rsSrc, netip.MustParseAddr("fe80::1"), rs(rsSrc, netip.MustParseAddr("fe80::1"), 0, [4]byte{}), nil},
		{"unspecified source without options", unspecified, rsAllRouters, rs(unspecified, rsAllRouters, 0, [4]byte{}), nil},
		{"unspecified source with unrecognized option", unspecified, rsAllRouters, rs(unspecified, rsAllRouters, 0, [4]byte{}, []byte{200, 1, 0, 0, 0, 0, 0, 0}), nil},
		{"invalid checksum", rsSrc, rsAllRouters, flipLast(rs(rsSrc, rsAllRouters, 0, [4]byte{})), ndp.ErrRSChecksum},
		{"checksum over another destination", rsSrc, rsAllRouters, rs(rsSrc, netip.MustParseAddr("ff02::1"), 0, [4]byte{}), ndp.ErrRSChecksum},
		{"checksum over another source", netip.MustParseAddr("fe80::3"), rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}), ndp.ErrRSChecksum},
		{"nonzero code", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 1, [4]byte{}), ndp.ErrRSMalformed},
		{"shorter than 8 octets", rsSrc, rsAllRouters, []byte{ndp.ICMPv6TypeRouterSolicitation, 0, 0, 0}, ndp.ErrRSMalformed},
		{"not a Router Solicitation", rsSrc, rsAllRouters, []byte{ndp.ICMPv6TypeRouterAdvertisement, 0, 0, 0, 0, 0, 0, 0}, ndp.ErrRSMalformed},
		{"option with zero length", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}, []byte{ndp.NDPOptionSourceLinkLayerAddress, 0, 0, 0, 0, 0, 0, 0}), ndp.ErrRSOptionLength},
		{"zero-length option after a valid one", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}, sllao(), []byte{200, 0, 0, 0, 0, 0, 0, 0}), ndp.ErrRSOptionLength},
		{"option past the end of the message", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}, []byte{200, 2, 0, 0, 0, 0, 0, 0}), ndp.ErrRSOptionLength},
		{"one trailing octet", rsSrc, rsAllRouters, rs(rsSrc, rsAllRouters, 0, [4]byte{}, []byte{200}), ndp.ErrRSOptionLength},
		{"unspecified source with source link-layer address", unspecified, rsAllRouters, rs(unspecified, rsAllRouters, 0, [4]byte{}, sllao()), ndp.ErrRSUnspecifiedSourceLLAddr},
		{"unspecified source with source link-layer address after another option", unspecified, rsAllRouters, rs(unspecified, rsAllRouters, 0, [4]byte{}, []byte{200, 1, 0, 0, 0, 0, 0, 0}, sllao()), ndp.ErrRSUnspecifiedSourceLLAddr},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ndp.ValidateRouterSolicitation(tc.src, tc.dst, tc.msg); !errors.Is(err, tc.want) {
				t.Fatalf("ValidateRouterSolicitation = %v, want %v", err, tc.want)
			}
		})
	}
}
