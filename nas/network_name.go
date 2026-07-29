// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"unicode/utf16"
)

// encodeGSM7 encodes a network name into the Network Name IE value
// (TS 24.008 §10.5.3.5a, referenced identically by TS 24.301 and TS 24.501): a
// coding-scheme octet followed by the name in the GSM 7-bit default alphabet
// (TS 23.038), packed, with no country initials. Characters are masked to 7
// bits, so the input is expected to be ASCII.
func encodeGSM7(name string) []byte {
	chars := len(name)
	packedLen := (chars*7 + 7) / 8
	spareBits := uint8(packedLen*8 - chars*7)

	out := make([]byte, 1+packedLen)
	// Octet 1: ext=1 (bit 8), coding scheme=GSM 7-bit (bits 7-5 = 000), add-CI=0
	// (bit 4), number of spare bits in the last octet (bits 3-1).
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

// parseGSM7 decodes a Network Name IE value encoded in the GSM 7-bit default
// alphabet. It returns an error for an empty value or a coding scheme
// other than GSM 7-bit (TS 24.008 §10.5.3.5a).
func parseGSM7(value []byte) (string, error) {
	if len(value) < 1 {
		return "", fmt.Errorf("network name: empty value")
	}

	if (value[0]>>4)&0x07 != 0 {
		return "", fmt.Errorf("network name: unsupported coding scheme")
	}

	spareBits := int(value[0] & 0x07)
	packed := value[1:]

	// The spare-bit count names bits of the last packed octet, so it cannot claim
	// more than the value carries: an element that says otherwise is malformed,
	// and computing a character count from it would go negative.
	if spareBits > len(packed)*8 {
		return "", fmt.Errorf("network name: %d spare bits in %d octets", spareBits, len(packed))
	}

	chars := (len(packed)*8 - spareBits) / 7

	out := make([]byte, 0, chars)
	bit := 0

	for i := 0; i < chars; i++ {
		pos, off := bit/8, bit%8
		c := packed[pos] >> uint(off)

		if off > 1 && pos+1 < len(packed) {
			c |= packed[pos+1] << uint(8-off)
		}

		out = append(out, c&0x7f)
		bit += 7
	}

	return string(out), nil
}

// NetworkNameCoding is the alphabet a network name is carried in
// (TS 24.008 table 10.5.94, octet 3 bits 7-5).
type NetworkNameCoding uint8

const (
	// NetworkNameGSM7Bit is the GSM default alphabet of TS 23.038, packed 7 bits
	// to a character.
	NetworkNameGSM7Bit NetworkNameCoding = 0
	// NetworkNameUCS2 is UTF-16, two octets to a code unit.
	NetworkNameUCS2 NetworkNameCoding = 1
)

func (c NetworkNameCoding) String() string {
	switch c {
	case NetworkNameGSM7Bit:
		return "GSM 7 bit"
	case NetworkNameUCS2:
		return "UCS2"
	default:
		return fmt.Sprintf("reserved coding scheme (%d)", uint8(c))
	}
}

// NetworkName is the network name information element value (TS 24.008
// §10.5.3.5a), which TS 24.301 §9.9.3.24 and TS 24.501 §9.11.3.35 both carry
// verbatim: the operator name a network offers the UE for display.
//
// Coding and AddCI are part of the value rather than of the encoding, because
// the element carries them and a receiver that dropped them could not put the
// name back on the wire as it arrived.
type NetworkName struct {
	Name string
	// AddCI asks the UE to prefix the name with the country's initials and a
	// separator (TS 24.008 table 10.5.94, octet 3 bit 4).
	AddCI bool
	// Coding is the alphabet the name travels in.
	Coding NetworkNameCoding
}

// NewNetworkName builds a network name in the narrowest alphabet that carries it
// exactly: the GSM default alphabet where every character is ASCII, UCS2
// otherwise.
func NewNetworkName(name string) NetworkName {
	for _, r := range name {
		if r > 0x7F {
			return NetworkName{Name: name, Coding: NetworkNameUCS2}
		}
	}

	return NetworkName{Name: name, Coding: NetworkNameGSM7Bit}
}

func (n NetworkName) String() string { return n.Name }

// ParseNetworkName decodes a network name IE value.
func ParseNetworkName(value []byte) (NetworkName, error) {
	if len(value) < 1 {
		return NetworkName{}, fmt.Errorf("network name: empty value")
	}

	n := NetworkName{
		AddCI:  value[0]&0x08 != 0,
		Coding: NetworkNameCoding(value[0] >> 4 & 0x07),
	}

	switch n.Coding {
	case NetworkNameGSM7Bit:
		name, err := parseGSM7(value)
		if err != nil {
			return NetworkName{}, err
		}

		n.Name = name
	case NetworkNameUCS2:
		body := value[1:]
		if len(body)%2 != 0 {
			return NetworkName{}, fmt.Errorf("network name: UCS2 body is %d octets, want an even count", len(body))
		}

		units := make([]uint16, len(body)/2)
		for i := range units {
			units[i] = uint16(body[2*i])<<8 | uint16(body[2*i+1])
		}

		n.Name = string(utf16.Decode(units))
	default:
		return NetworkName{}, fmt.Errorf("network name: %s", n.Coding)
	}

	return n, nil
}

// AppendBinary encodes the network name IE value onto b.
func (n NetworkName) AppendBinary(b []byte) ([]byte, error) {
	var addCI byte
	if n.AddCI {
		addCI = 0x08
	}

	switch n.Coding {
	case NetworkNameGSM7Bit:
		raw := encodeGSM7(n.Name)
		raw[0] |= addCI

		return append(b, raw...), nil
	case NetworkNameUCS2:
		// TS 24.008 §10.5.3.5a: outside the GSM alphabet the spare-bit count
		// carries no information, so it is written as zero.
		b = append(b, 0x80|uint8(NetworkNameUCS2)<<4|addCI)

		for _, u := range utf16.Encode([]rune(n.Name)) {
			b = append(b, uint8(u>>8), uint8(u))
		}

		return b, nil
	default:
		return b, fmt.Errorf("network name: cannot encode a %s", n.Coding)
	}
}

// MarshalBinary encodes the network name IE value.
func (n NetworkName) MarshalBinary() ([]byte, error) { return n.AppendBinary(nil) }
