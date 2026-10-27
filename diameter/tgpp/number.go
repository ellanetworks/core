// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter/tbcd"
)

var ErrInvalidNumber = errors.New("tgpp: invalid number")

const (
	maxE164Digits = 15
	minIMSIDigits = 6
	maxIMSIDigits = 15
)

func EncodeE164(number string) ([]byte, error) {
	if !ValidE164(number) {
		return nil, fmt.Errorf("%w: E.164 number %q", ErrInvalidNumber, number)
	}

	return tbcd.Encode(number)
}

func DecodeE164(b []byte) (string, error) {
	number, err := tbcd.Decode(b)
	if err != nil {
		return "", err
	}

	if !ValidE164(number) {
		return "", fmt.Errorf("%w: E.164 number %q", ErrInvalidNumber, number)
	}

	return number, nil
}

func ValidE164(number string) bool {
	return Digits(number, 1, maxE164Digits)
}

func ValidIMSI(imsi string) bool {
	return Digits(imsi, minIMSIDigits, maxIMSIDigits)
}

func Digits(s string, minLen, maxLen int) bool {
	return len(s) >= minLen && len(s) <= maxLen && decimal(s)
}

func decimal(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}
