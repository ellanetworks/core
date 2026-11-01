// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tpdu

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
		return 0, fmt.Errorf("tpdu: %q is not a digit", c)
	}

	return c - '0', nil
}

func decodeAddress(b []byte) (string, int, error) {
	if len(b) < 2 {
		return "", 0, fmt.Errorf("tpdu: truncated address")
	}

	count := int(b[0])
	size := 2 + (count+1)/2

	if len(b) < size {
		return "", 0, fmt.Errorf("tpdu: truncated address")
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

// PIDSIMDataDownload is the TP-PID of a (U)SIM Data download (TS 23.040 clause 9.2.3.9): bits 7-6 '01', then
// '111111'.
const PIDSIMDataDownload = 0x7f

// Charset is the character set of the TP-UD that a TP-DCS tells (TS 23.038 clause 4).
type Charset int

const (
	CharsetGSM7 Charset = iota
	Charset8Bit
	CharsetUCS2
)

// CharsetOf is the character set of TP-DCS dcs (TS 23.038 clause 4). Reserved codings are the GSM 7 bit default
// alphabet, as the clause requires.
func CharsetOf(dcs byte) Charset {
	switch {
	case dcs&0x80 == 0: // 00xx, 01xx: General Data Coding indication, bits 3-2.
		switch dcs >> 2 & 0x03 {
		case 0x01:
			return Charset8Bit
		case 0x02:
			return CharsetUCS2
		default:
			return CharsetGSM7
		}
	case dcs&0xf0 == 0xe0: // 1110: Message Waiting Indication Group, UCS2.
		return CharsetUCS2
	case dcs&0xf0 == 0xf0: // 1111: Data coding/message class, bit 2.
		if dcs&0x04 != 0 {
			return Charset8Bit
		}

		return CharsetGSM7
	default:
		return CharsetGSM7
	}
}

// MessageClass is the message class of TP-DCS dcs, 0 to 3, if it has one (TS 23.038 clause 4): in the General Data
// Coding groups when bit 4 is set, and in the Data coding/message class group.
func MessageClass(dcs byte) (int, bool) {
	switch {
	case dcs&0x80 == 0 && dcs&0x10 != 0, dcs&0xf0 == 0xf0:
		return int(dcs & 0x03), true
	default:
		return 0, false
	}
}

// Deliver is an SMS-DELIVER (TS 23.040 clause 9.2.2.1), but for the TP-MMS, TP-LP, TP-SRI and TP-RP flags.
type Deliver struct {
	// UDHI is TP-UDHI: the TP-UD starts with a user data header.
	UDHI bool
	// OriginatingAddress is TP-OA, with a leading "+" when international.
	OriginatingAddress string
	PID                byte
	DCS                byte
	// SCTS is TP-SCTS, as received.
	SCTS [7]byte
	// UDL is TP-UDL: septets in the GSM 7 bit default alphabet, octets otherwise (TS 23.040 clause 9.2.3.16).
	UDL int
	// UD is TP-UD, its user data header included.
	UD []byte
}

// DecodeDeliver decodes an SMS-DELIVER (TS 23.040 clause 9.2.2.1).
func DecodeDeliver(b []byte) (Deliver, error) {
	if len(b) < 1 || b[0]&0x03 != 0x00 {
		return Deliver{}, fmt.Errorf("tpdu: not an SMS-DELIVER")
	}

	d := Deliver{UDHI: b[0]&0x40 != 0}

	from, size, err := decodeAddress(b[1:])
	if err != nil {
		return Deliver{}, err
	}

	d.OriginatingAddress = from

	// TP-PID 1, TP-DCS 1, TP-SCTS 7, TP-UDL 1.
	i := 1 + size
	if len(b) < i+10 {
		return Deliver{}, fmt.Errorf("tpdu: truncated SMS-DELIVER")
	}

	d.PID, d.DCS = b[i], b[i+1]
	copy(d.SCTS[:], b[i+2:i+9])
	d.UDL = int(b[i+9])
	d.UD = b[i+10:]

	octets := d.UDL
	if CharsetOf(d.DCS) == CharsetGSM7 {
		octets = (7*d.UDL + 7) / 8
	}

	if len(d.UD) < octets {
		return Deliver{}, fmt.Errorf("tpdu: TP-UDL %d exceeds %d octets of TP-UD", d.UDL, len(d.UD))
	}

	d.UD = d.UD[:octets]

	return d, nil
}

// Text is the text of d's TP-UD, without its user data header, in the character set of its TP-DCS.
func (d Deliver) Text() (string, error) {
	headerOctets := 0
	if d.UDHI && len(d.UD) > 0 {
		headerOctets = int(d.UD[0]) + 1
	}

	if headerOctets > len(d.UD) {
		return "", fmt.Errorf("tpdu: user data header of %d octets in %d", headerOctets, len(d.UD))
	}

	switch CharsetOf(d.DCS) {
	case CharsetGSM7:
		fill := (7 - headerOctets*8%7) % 7
		skip := (headerOctets*8 + fill) / 7

		septets := unpackSeptets(d.UD[headerOctets:], d.UDL-skip, fill)
		alphabet := []rune(gsm7Basic)

		var sb strings.Builder
		for _, v := range septets {
			sb.WriteRune(alphabet[v])
		}

		return sb.String(), nil
	case CharsetUCS2:
		body := d.UD[headerOctets:]
		units := make([]uint16, len(body)/2)

		for k := range units {
			units[k] = binary.BigEndian.Uint16(body[2*k:])
		}

		return string(utf16.Decode(units)), nil
	default:
		return string(d.UD[headerOctets:]), nil
	}
}

// MaxDeliverReportUserData is the most octets of TP-UD in an SMS-DELIVER-REPORT for RP-ACK (TS 23.040 clause
// 9.2.2.1a).
const MaxDeliverReportUserData = 159

// DeliverReportAck is an SMS-DELIVER-REPORT for RP-ACK (TS 23.040 clause 9.2.2.1a), with 8-bit or UCS2 user data.
type DeliverReportAck struct {
	UDHI bool
	// PID and DCS are TP-PID and TP-DCS, when present.
	PID, DCS *byte
	// UD is TP-UD; TP-UDL is present when it is not nil.
	UD []byte
}

// AppendBinary appends r to b. The TP-PI tells which of TP-PID, TP-DCS and TP-UDL are present (TS 23.040 clause
// 9.2.3.27).
func (r DeliverReportAck) AppendBinary(b []byte) ([]byte, error) {
	if len(r.UD) > MaxDeliverReportUserData {
		return nil, fmt.Errorf("tpdu: %d octets of TP-UD exceed %d", len(r.UD), MaxDeliverReportUserData)
	}

	var dcs byte
	if r.DCS != nil {
		dcs = *r.DCS
	}

	// Without a TP-DCS, the TP-UD is in the GSM 7 bit default alphabet, of TP-UDL in septets.
	if r.UD != nil && CharsetOf(dcs) == CharsetGSM7 {
		return nil, fmt.Errorf("tpdu: user data in the GSM 7 bit default alphabet is not supported")
	}

	// TP-MTI '00' and TP-UDHI; bits 7 and 5-2 are zero.
	first := byte(0x00)
	if r.UDHI {
		first |= 0x40
	}

	var pi byte

	if r.PID != nil {
		pi |= 0x01
	}

	if r.DCS != nil {
		pi |= 0x02
	}

	if r.UD != nil {
		pi |= 0x04
	}

	b = append(b, first, pi)

	if r.PID != nil {
		b = append(b, *r.PID)
	}

	if r.DCS != nil {
		b = append(b, *r.DCS)
	}

	if r.UD != nil {
		b = append(b, byte(len(r.UD)))
		b = append(b, r.UD...)
	}

	return b, nil
}
