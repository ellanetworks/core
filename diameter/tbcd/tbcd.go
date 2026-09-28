// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tbcd

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalid = errors.New("tbcd: invalid TBCD string")

const (
	alphabet = "0123456789*#abc"
	filler   = 0xf
)

func Decode(b []byte) (string, error) {
	if len(b) == 0 {
		return "", fmt.Errorf("%w: empty", ErrInvalid)
	}

	var digits strings.Builder

	digits.Grow(2 * len(b))

	for i, octet := range b {
		low, high := octet&0xf, octet>>4

		if low == filler {
			return "", fmt.Errorf("%w: filler in low nibble of octet %d", ErrInvalid, i)
		}

		digits.WriteByte(alphabet[low])

		if high == filler {
			if i != len(b)-1 {
				return "", fmt.Errorf("%w: filler in octet %d", ErrInvalid, i)
			}

			break
		}

		digits.WriteByte(alphabet[high])
	}

	return digits.String(), nil
}

func Encode(digits string) ([]byte, error) {
	if digits == "" {
		return nil, fmt.Errorf("%w: empty", ErrInvalid)
	}

	out := make([]byte, 0, (len(digits)+1)/2)

	for i := 0; i < len(digits); i += 2 {
		low, err := nibble(digits[i])
		if err != nil {
			return nil, err
		}

		high := byte(filler)

		if i+1 < len(digits) {
			if high, err = nibble(digits[i+1]); err != nil {
				return nil, err
			}
		}

		out = append(out, high<<4|low)
	}

	return out, nil
}

func nibble(c byte) (byte, error) {
	if c >= 'A' && c <= 'C' {
		c += 'a' - 'A'
	}

	i := strings.IndexByte(alphabet, c)
	if i < 0 {
		return 0, fmt.Errorf("%w: character %q", ErrInvalid, c)
	}

	return byte(i), nil
}
