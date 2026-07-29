// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "fmt"

// EncodeBCDIdentity packs the decimal digits of a BCD mobile identity (IMSI,
// IMEI or IMEISV, TS 24.008 §10.5.1.4, identical in TS 24.501 §9.11.3.4) into the
// IE value: octet 1 carries identity digit 1 in bits 5-8, the odd/even indicator
// in bit 4, and typeOfIdentity in bits 1-3; the remaining digits are TBCD-packed
// two per octet, with a 0xF filler nibble when the total digit count is even.
func EncodeBCDIdentity(digits string, typeOfIdentity uint8) ([]byte, error) {
	if len(digits) == 0 {
		return nil, fmt.Errorf("mobile identity: empty digits")
	}

	if digits[0] < '0' || digits[0] > '9' {
		return nil, fmt.Errorf("mobile identity: first character %q is not a digit", digits[0])
	}

	rest, err := EncodeTBCD(digits[1:])
	if err != nil {
		return nil, err
	}

	oddEven := byte(len(digits) & 1)
	head := (digits[0]-'0')<<4 | oddEven<<3 | typeOfIdentity&0x07

	return append([]byte{head}, rest...), nil
}

// DecodeBCDIdentity returns the digit string of a BCD mobile identity IE value,
// the inverse of EncodeBCDIdentity: identity digit 1 from octet 1 bits 5-8
// followed by the TBCD-packed remaining digits (TS 24.008 §10.5.1.4). An empty
// value yields the empty string.
func DecodeBCDIdentity(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}

	if b[0]>>4 > 9 {
		return "", &Error{Op: "BCD identity digit", Err: ErrDigit}
	}

	rest, err := DecodeTBCD(b[1:])
	if err != nil {
		return "", err
	}

	return string([]byte{'0' + (b[0] >> 4)}) + rest, nil
}
