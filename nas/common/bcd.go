// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

// DecodeTBCD decodes a telephony-BCD octet string (TS 24.008): two
// digits per octet, least-significant nibble first. A 0xF nibble is the
// odd-length filler and is skipped. Only decimal digits are produced (sufficient
// for EPS identities such as IMSI/IMEI).
func DecodeTBCD(b []byte) string {
	out := make([]byte, 0, len(b)*2)

	for _, o := range b {
		for _, nib := range [2]byte{o & 0x0F, o >> 4} {
			if nib == 0x0F {
				continue
			}

			if nib > 9 {
				continue
			}

			out = append(out, '0'+nib)
		}
	}

	return string(out)
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

// EncodePLMN packs an MCC (3 digits) and MNC (2 or 3 digits) into the 3-octet
// PLMN identity of TS 24.301 / TS 24.008. A 2-digit MNC sets
// the MNC-digit-3 nibble to 0xF.
func EncodePLMN(mcc, mnc string) ([3]byte, error) {
	var out [3]byte

	if len(mcc) != 3 || (len(mnc) != 2 && len(mnc) != 3) {
		return out, &Error{Op: "PLMN length", Err: ErrDigit}
	}

	d := make([]byte, 0, 6)

	for _, s := range []string{mcc, mnc} {
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return out, &Error{Op: "PLMN digit", Offset: i, Err: ErrDigit}
			}

			d = append(d, s[i]-'0')
		}
	}

	out[0] = d[1]<<4 | d[0] // MCC2 | MCC1

	if len(mnc) == 2 {
		out[1] = 0xF0 | d[2]    // 1111 | MCC3
		out[2] = d[4]<<4 | d[3] // MNC2 | MNC1
	} else {
		out[1] = d[5]<<4 | d[2] // MNC3 | MCC3
		out[2] = d[4]<<4 | d[3] // MNC2 | MNC1
	}

	return out, nil
}

// DecodePLMN reverses EncodePLMN. A 0xF MNC-digit-3 nibble yields a 2-digit MNC.
func DecodePLMN(b [3]byte) (mcc, mnc string) {
	mcc = string([]byte{'0' + (b[0] & 0x0F), '0' + (b[0] >> 4), '0' + (b[1] & 0x0F)})

	if hi := b[1] >> 4; hi == 0x0F {
		mnc = string([]byte{'0' + (b[2] & 0x0F), '0' + (b[2] >> 4)})
	} else {
		mnc = string([]byte{'0' + (b[2] & 0x0F), '0' + (b[2] >> 4), '0' + hi})
	}

	return mcc, mnc
}
