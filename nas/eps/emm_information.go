// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// MsgEMMInformation is the EMM INFORMATION message type (TS 24.301).
const MsgEMMInformation MessageType = 0x61

// IEIs of the network-name information elements in EMM INFORMATION
// (TS 24.301).
const (
	fullNameForNetworkIEI  uint8 = 0x43
	shortNameForNetworkIEI uint8 = 0x45
)

// EMMInformation is the EMM INFORMATION message (TS 24.301), sent by the
// MME to provide the network name to the UE. The procedure is optional in the
// network; the MME sends it integrity-protected and ciphered after
// attach. Only the network-name IEs are carried.
type EMMInformation struct {
	FullNetworkName  string
	ShortNetworkName string
}

// Marshal encodes the plain EMM INFORMATION message.
func (m *EMMInformation) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgEMMInformation)

	if m.FullNetworkName != "" {
		w.U8(fullNameForNetworkIEI)

		if err := w.LV(encodeNetworkName(m.FullNetworkName)); err != nil {
			return nil, err
		}
	}

	if m.ShortNetworkName != "" {
		w.U8(shortNameForNetworkIEI)

		if err := w.LV(encodeNetworkName(m.ShortNetworkName)); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// encodeNetworkName encodes a network name into the Network name IE value
// (TS 24.301 ≡ TS 24.008): a coding-scheme octet followed by
// the name in the GSM 7-bit default alphabet (TS 23.038), packed, with no
// country initials. Characters are masked to 7 bits, so the input is expected to
// be ASCII.
func encodeNetworkName(name string) []byte {
	chars := len(name)
	packedLen := (chars*7 + 7) / 8
	spareBits := uint8(packedLen*8 - chars*7)

	out := make([]byte, 1+packedLen)
	// Octet 1: ext=1, coding scheme=GSM 7-bit (000), add-CI=0, number of spare
	// bits in the last octet (bits 3-1).
	out[0] = 0x80 | (spareBits & 0x07)

	bit := 0

	for i := 0; i < chars; i++ {
		c := name[i] & 0x7f
		pos, off := bit/8, bit%8

		out[1+pos] |= c << uint(off)
		if off > 1 {
			out[1+pos+1] |= c >> uint(8-off)
		}

		bit += 7
	}

	return out
}
