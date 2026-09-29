// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsue

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

const (
	dcsGSM7 = 0x00
	dcsUCS2 = 0x08
)

var gsm7Basic = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞ\x1bÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"

func gsm7Septets(text string) ([]byte, bool) {
	alphabet := []rune(gsm7Basic)
	out := make([]byte, 0, len(text))

	for _, r := range text {
		i := indexRune(alphabet, r)
		if i < 0 || r == '\x1b' {
			return nil, false
		}

		out = append(out, byte(i))
	}

	return out, true
}

func indexRune(alphabet []rune, r rune) int {
	for i, a := range alphabet {
		if a == r {
			return i
		}
	}

	return -1
}

func packSeptets(septets []byte, fill int) []byte {
	bits := fill + len(septets)*7
	out := make([]byte, (bits+7)/8)

	for i, v := range septets {
		pos := fill + i*7
		out[pos/8] |= v << (pos % 8)

		if pos%8 > 1 {
			out[pos/8+1] |= v >> (8 - pos%8)
		}
	}

	return out
}

func unpackSeptets(b []byte, count, fill int) []byte {
	out := make([]byte, 0, count)

	for i := range count {
		pos := fill + i*7

		v := b[pos/8] >> (pos % 8)
		if pos%8 > 1 && pos/8+1 < len(b) {
			v |= b[pos/8+1] << (8 - pos%8)
		}

		out = append(out, v&0x7f)
	}

	return out
}

func encodeAddress(digits string) ([]byte, error) {
	digits = strings.TrimPrefix(digits, "+")

	out := []byte{byte(len(digits)), 0x91}

	for i := 0; i < len(digits); i += 2 {
		lo, err := digit(digits[i])
		if err != nil {
			return nil, err
		}

		hi := byte(0x0f)

		if i+1 < len(digits) {
			if hi, err = digit(digits[i+1]); err != nil {
				return nil, err
			}
		}

		out = append(out, hi<<4|lo)
	}

	return out, nil
}

func digit(c byte) (byte, error) {
	if c < '0' || c > '9' {
		return 0, fmt.Errorf("smsue: %q is not a digit", c)
	}

	return c - '0', nil
}

func decodeAddress(b []byte) (string, int, error) {
	if len(b) < 2 {
		return "", 0, fmt.Errorf("smsue: truncated address")
	}

	count := int(b[0])
	size := 2 + (count+1)/2

	if len(b) < size {
		return "", 0, fmt.Errorf("smsue: truncated address")
	}

	var sb strings.Builder

	if b[1]&0x70 == 0x10 {
		sb.WriteByte('+')
	}

	for i := range count {
		v := b[2+i/2]
		if i%2 == 1 {
			v >>= 4
		}

		sb.WriteByte('0' + v&0x0f)
	}

	return sb.String(), size, nil
}

func EncodeSubmit(reference uint8, to, text string) ([]byte, error) {
	address, err := encodeAddress(to)
	if err != nil {
		return nil, err
	}

	out := append([]byte{0x01, reference}, address...)

	if septets, ok := gsm7Septets(text); ok {
		out = append(out, 0x00, dcsGSM7, byte(len(septets)))
		return append(out, packSeptets(septets, 0)...), nil
	}

	units := utf16.Encode([]rune(text))
	ud := make([]byte, 2*len(units))

	for i, u := range units {
		binary.BigEndian.PutUint16(ud[2*i:], u)
	}

	out = append(out, 0x00, dcsUCS2, byte(len(ud)))

	return append(out, ud...), nil
}

func DecodeDeliver(tpdu []byte) (from, text string, err error) {
	if len(tpdu) < 1 || tpdu[0]&0x03 != 0x00 {
		return "", "", fmt.Errorf("smsue: not an SMS-DELIVER")
	}

	udhi := tpdu[0]&0x40 != 0

	from, size, err := decodeAddress(tpdu[1:])
	if err != nil {
		return "", "", err
	}

	i := 1 + size
	if len(tpdu) < i+10 {
		return "", "", fmt.Errorf("smsue: truncated SMS-DELIVER")
	}

	dcs := tpdu[i+1]
	udl := int(tpdu[i+9])
	ud := tpdu[i+10:]

	headerOctets := 0
	if udhi && len(ud) > 0 {
		headerOctets = int(ud[0]) + 1
	}

	switch dcs & 0x0c {
	case 0x00:
		fill := (7 - headerOctets*8%7) % 7
		skip := (headerOctets*8 + fill) / 7

		septets := unpackSeptets(ud[headerOctets:], udl-skip, fill)
		alphabet := []rune(gsm7Basic)

		var sb strings.Builder
		for _, v := range septets {
			sb.WriteRune(alphabet[v])
		}

		return from, sb.String(), nil
	case 0x08:
		body := ud[headerOctets:udl]
		units := make([]uint16, len(body)/2)

		for k := range units {
			units[k] = binary.BigEndian.Uint16(body[2*k:])
		}

		return from, string(utf16.Decode(units)), nil
	default:
		return from, string(ud[headerOctets:udl]), nil
	}
}
