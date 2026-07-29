// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

// DecodeTBCD decodes a telephony-BCD octet string (TS 24.008): two digits per
// octet, least-significant nibble first, the digits followed by 0xF fillers to
// the end of the string.
//
// It rejects any other nibble outside 0-9, and any digit after a filler: a TBCD
// digit is decimal, so such an octet string could not have been encoded from any
// digit string and must not decode to a value that cannot be re-encoded.
func DecodeTBCD(b []byte) (string, error) {
	out := make([]byte, 0, len(b)*2)
	filled := false

	for i, o := range b {
		for half, nib := range [2]byte{o & 0x0F, o >> 4} {
			switch {
			case nib == 0x0F:
				filled = true
			case nib > 9 || filled:
				return "", &Error{Op: "TBCD digit", Offset: i*2 + half, Err: ErrDigit}
			default:
				out = append(out, '0'+nib)
			}
		}
	}

	return string(out), nil
}

// EncodeTBCD packs decimal digits two per octet, least-significant nibble first,
// padding an odd count with a 0xF filler in the final high nibble.
func EncodeTBCD(digits string) ([]byte, error) {
	out := make([]byte, (len(digits)+1)/2)

	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return nil, &Error{Op: "TBCD digit", Offset: i, Err: ErrDigit}
		}

		v := digits[i] - '0'

		if i%2 == 0 {
			out[i/2] = v
		} else {
			out[i/2] |= v << 4
		}
	}

	if len(digits)%2 == 1 {
		out[len(out)-1] |= 0xF0
	}

	return out, nil
}

// PLMN is a public land mobile network identity: a 3-digit mobile country code
// and a 2- or 3-digit mobile network code (TS 23.003 §2.2), carried as three
// octets wherever an element names a network — TAI, GUTI, SUCI.
type PLMN struct {
	MCC string // 3 digits
	MNC string // 2 or 3 digits
}

// String renders the identity as "mcc-mnc".
func (p PLMN) String() string { return p.MCC + "-" + p.MNC }

// Valid reports whether the identity has the digit counts TS 23.003 §2.2 fixes
// and no non-decimal digit.
func (p PLMN) Valid() bool {
	if len(p.MCC) != 3 || (len(p.MNC) != 2 && len(p.MNC) != 3) {
		return false
	}

	for _, s := range []string{p.MCC, p.MNC} {
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return false
			}
		}
	}

	return true
}

// ParsePLMN decodes a 3-octet PLMN identity. A 0xF MNC-digit-3 nibble yields a
// 2-digit MNC.
//
// It rejects any other nibble outside 0-9: an MCC or MNC digit is decimal
// (TS 23.003 §2.2), so such an identity could not have been encoded from any
// MCC/MNC pair and must not decode to a value that cannot be re-encoded.
func ParsePLMN(b [3]byte) (PLMN, error) {
	digits := [6]uint8{b[0] & 0x0F, b[0] >> 4, b[1] & 0x0F, b[2] & 0x0F, b[2] >> 4, b[1] >> 4}

	twoDigitMNC := digits[5] == 0x0F
	if twoDigitMNC {
		digits[5] = 0
	}

	for i, d := range digits {
		if d > 9 {
			return PLMN{}, &Error{Op: "PLMN digit", Offset: i, Err: ErrDigit}
		}
	}

	p := PLMN{MCC: string([]byte{'0' + digits[0], '0' + digits[1], '0' + digits[2]})}

	if twoDigitMNC {
		p.MNC = string([]byte{'0' + digits[3], '0' + digits[4]})
	} else {
		p.MNC = string([]byte{'0' + digits[3], '0' + digits[4], '0' + digits[5]})
	}

	return p, nil
}

// Octets packs the identity into the three octets an element carries. A 2-digit
// MNC sets the MNC-digit-3 nibble to 0xF.
func (p PLMN) Octets() ([3]byte, error) {
	var out [3]byte

	if !p.Valid() {
		return out, &Error{Op: "PLMN", Err: ErrDigit}
	}

	d := make([]byte, 0, 6)

	for _, s := range []string{p.MCC, p.MNC} {
		for i := 0; i < len(s); i++ {
			d = append(d, s[i]-'0')
		}
	}

	out[0] = d[1]<<4 | d[0] // MCC2 | MCC1

	if len(p.MNC) == 2 {
		out[1] = 0xF0 | d[2]    // 1111 | MCC3
		out[2] = d[4]<<4 | d[3] // MNC2 | MNC1
	} else {
		out[1] = d[5]<<4 | d[2] // MNC3 | MCC3
		out[2] = d[4]<<4 | d[3] // MNC2 | MNC1
	}

	return out, nil
}

// AppendBinary encodes the PLMN identity onto b.
func (p PLMN) AppendBinary(b []byte) ([]byte, error) {
	o, err := p.Octets()
	if err != nil {
		return b, err
	}

	return append(b, o[:]...), nil
}

// MarshalBinary encodes the PLMN identity.
func (p PLMN) MarshalBinary() ([]byte, error) { return p.AppendBinary(nil) }
