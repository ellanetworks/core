// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package etsi

import "fmt"

// supiType distinguishes the form of a SUPI.
type supiType int

const (
	supiTypeUnset supiType = iota // zero value = not set
	supiTypeIMSI
	supiTypeNAI
)

// SUPI is a 5G Subscription Permanent Identifier (TS 23.501), in either IMSI
// (International Mobile Subscriber Identity) or NAI (Network Access Identifier)
// form. Only the IMSI form is supported. The zero value is an unset identity.
type SUPI struct {
	supiType supiType
	value    string // bare numeric IMSI (e.g. "001019756139935") or NAI string
}

// InvalidSUPI is the zero value of SUPI, an unset identity.
var InvalidSUPI SUPI = SUPI{}

const imsiPrefix = "imsi-"

// NewSUPIFromIMSI creates a SUPI from a bare 15-digit numeric IMSI.
func NewSUPIFromIMSI(imsi string) (SUPI, error) {
	if err := validateIMSI(imsi); err != nil {
		return InvalidSUPI, err
	}

	return SUPI{supiType: supiTypeIMSI, value: imsi}, nil
}

// NewSUPIFromPrefixed creates a SUPI from an IMSI string with or without the
// "imsi-" prefix; both forms normalize to the same identity.
func NewSUPIFromPrefixed(s string) (SUPI, error) {
	if len(s) >= len(imsiPrefix) && s[:len(imsiPrefix)] == imsiPrefix {
		return NewSUPIFromIMSI(s[len(imsiPrefix):])
	}

	return NewSUPIFromIMSI(s)
}

// NewSUPIFromNAI creates a SUPI from a Network Access Identifier; not yet supported.
func NewSUPIFromNAI(_ string) (SUPI, error) {
	return InvalidSUPI, fmt.Errorf("NAI SUPI not yet supported")
}

// String returns the prefixed form: "imsi-<digits>", "nai-<value>", or "" when unset.
func (s SUPI) String() string {
	switch s.supiType {
	case supiTypeIMSI:
		return imsiPrefix + s.value
	case supiTypeNAI:
		return "nai-" + s.value
	default:
		return ""
	}
}

// IMSI returns the bare numeric IMSI digits. It panics if the SUPI is not
// IMSI-based, so check IsIMSI first.
func (s SUPI) IMSI() string {
	if s.supiType != supiTypeIMSI {
		panic("IMSI() called on non-IMSI SUPI")
	}

	return s.value
}

func (s SUPI) IsValid() bool {
	return s.supiType != supiTypeUnset
}

func (s SUPI) IsIMSI() bool {
	return s.supiType == supiTypeIMSI
}

func (s SUPI) IsNAI() bool {
	return s.supiType == supiTypeNAI
}

// validateIMSI checks that s is a valid IMSI: 15 digits, all numeric.
func validateIMSI(s string) error {
	if len(s) != 15 {
		return fmt.Errorf("invalid IMSI length: %d (must be 15 digits)", len(s))
	}

	if !isAllDigits(s) {
		return fmt.Errorf("invalid IMSI: contains non-digit characters: %s", s)
	}

	return nil
}

// isAllDigits returns true if every byte in s is an ASCII digit.
func isAllDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}
