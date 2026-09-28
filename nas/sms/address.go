// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"fmt"
	"strings"
)

// TypeOfNumber is the type of number of an RP address (TS 24.008 table 10.5.118).
type TypeOfNumber uint8

// Type of number values (TS 24.008 table 10.5.118).
const (
	TypeOfNumberUnknown         TypeOfNumber = 0
	TypeOfNumberInternational   TypeOfNumber = 1
	TypeOfNumberNational        TypeOfNumber = 2
	TypeOfNumberNetworkSpecific TypeOfNumber = 3
	TypeOfNumberDedicatedAccess TypeOfNumber = 4
)

// NumberingPlan is the numbering plan identification of an RP address
// (TS 24.008 table 10.5.118).
type NumberingPlan uint8

// Numbering plan identification values (TS 24.008 table 10.5.118).
const (
	NumberingPlanUnknown  NumberingPlan = 0
	NumberingPlanISDN     NumberingPlan = 1
	NumberingPlanData     NumberingPlan = 3
	NumberingPlanTelex    NumberingPlan = 4
	NumberingPlanNational NumberingPlan = 8
	NumberingPlanPrivate  NumberingPlan = 9
)

const (
	minAddressLen = 2
	maxAddressLen = 11
	bcdDigits     = "0123456789*#abc"
)

// Address is an RP-Originator or RP-Destination address value (TS 24.011
// §8.2.5.1, §8.2.5.2), coded as the called party BCD number contents
// (TS 24.008 §10.5.4.7).
type Address struct {
	TypeOfNumber  TypeOfNumber
	NumberingPlan NumberingPlan
	Digits        string

	Raw []byte
}

// E164Address returns an international E.164 address of the given digits.
func E164Address(digits string) *Address {
	return &Address{TypeOfNumber: TypeOfNumberInternational, NumberingPlan: NumberingPlanISDN, Digits: digits}
}

// String renders the digits, prefixed with "+" for an international number.
func (a Address) String() string {
	if a.Raw != nil {
		return fmt.Sprintf("undecodable address %x", a.Raw)
	}

	if a.TypeOfNumber == TypeOfNumberInternational {
		return "+" + a.Digits
	}

	return a.Digits
}

// ParseAddress decodes a non-empty RP address value. A value longer than the
// TS 24.011 maximum is accepted (§9.1).
func ParseAddress(b []byte) (Address, error) {
	if len(b) < minAddressLen {
		return Address{}, fmt.Errorf("sms: address is %d octets, want at least %d", len(b), minAddressLen)
	}

	a := Address{TypeOfNumber: TypeOfNumber(b[0] >> 4 & 0x07), NumberingPlan: NumberingPlan(b[0] & 0x0F)}

	var digits strings.Builder

	last := 2*(len(b)-1) - 1

	for i := 0; i <= last; i++ {
		nibble := b[1+i/2] >> (4 * (i % 2)) & 0x0F
		if nibble == 0x0F {
			if i != last {
				return Address{}, fmt.Errorf("sms: address end mark before the last digit")
			}

			continue
		}

		digits.WriteByte(bcdDigits[nibble])
	}

	a.Digits = digits.String()

	return a, nil
}

// AppendBinary encodes the address value onto b. Raw, when set, is written as
// it arrived.
func (a Address) AppendBinary(b []byte) ([]byte, error) {
	if a.Raw != nil {
		return append(b, a.Raw...), nil
	}

	if a.TypeOfNumber > 0x07 || a.NumberingPlan > 0x0F {
		return b, fmt.Errorf("sms: address type of number %d or numbering plan %d out of range", a.TypeOfNumber, a.NumberingPlan)
	}

	n := 1 + (len(a.Digits)+1)/2
	if n < minAddressLen {
		return b, fmt.Errorf("sms: address has no digits")
	}

	if n > maxAddressLen {
		return b, fmt.Errorf("sms: address of %d digits is %d octets, want at most %d: %w", len(a.Digits), n, maxAddressLen, ErrElementTooLong)
	}

	out := append(b, 0x80|uint8(a.TypeOfNumber)<<4|uint8(a.NumberingPlan))

	for i := 0; i < len(a.Digits); i += 2 {
		lo := strings.IndexByte(bcdDigits, a.Digits[i])
		if lo < 0 {
			return b, fmt.Errorf("sms: address digit %q", a.Digits[i])
		}

		hi := 0x0F

		if i+1 < len(a.Digits) {
			if hi = strings.IndexByte(bcdDigits, a.Digits[i+1]); hi < 0 {
				return b, fmt.Errorf("sms: address digit %q", a.Digits[i+1])
			}
		}

		out = append(out, uint8(hi)<<4|uint8(lo))
	}

	return out, nil
}

// MarshalBinary encodes the address value.
func (a Address) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }
