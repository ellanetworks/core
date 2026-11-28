// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ndp

import (
	"errors"
	"net/netip"
)

const rsHeaderLen = 8

var (
	ErrRSMalformed               = errors.New("not a Router Solicitation of 8 or more octets with code 0")
	ErrRSChecksum                = errors.New("invalid ICMPv6 checksum")
	ErrRSOptionLength            = errors.New("option with length zero or past the end of the message")
	ErrRSUnspecifiedSourceLLAddr = errors.New("source link-layer address option from the unspecified address")
)

func ValidateRouterSolicitation(src, dst netip.Addr, msg []byte) error {
	if len(msg) < rsHeaderLen || msg[0] != ICMPv6TypeRouterSolicitation || msg[1] != 0 {
		return ErrRSMalformed
	}

	if ICMPv6Checksum(src, dst, msg) != 0 {
		return ErrRSChecksum
	}

	for opts := msg[rsHeaderLen:]; len(opts) > 0; {
		if len(opts) < 2 {
			return ErrRSOptionLength
		}

		n := int(opts[1]) * 8
		if n == 0 || n > len(opts) {
			return ErrRSOptionLength
		}

		if opts[0] == NDPOptionSourceLinkLayerAddress && src.IsUnspecified() {
			return ErrRSUnspecifiedSourceLLAddr
		}

		opts = opts[n:]
	}

	return nil
}

var AllNodesMulticast = netip.MustParseAddr("ff02::1")

func SolicitedRADestination(rsSource netip.Addr) netip.Addr {
	if rsSource.IsUnspecified() {
		return AllNodesMulticast
	}

	return rsSource
}
