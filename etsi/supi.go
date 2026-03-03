package etsi

import "fmt"

// supiType distinguishes the form of a SUPI.
type supiType int

const (
	supiTypeUnset supiType = iota // zero value = not set
	supiTypeIMSI
	supiTypeNAI
)

// SUPI represents a 5G Subscription Permanent Identifier,
// as defined by 3GPP TS 23.501. A SUPI can take two forms:
// an IMSI (International Mobile Subscriber Identity) or an
// NAI (Network Access Identifier). This implementation fully
// supports the IMSI form; NAI support is stubbed for future use.
//
// The zero value of SUPI represents an unset identity.
// Use the constructor functions to create valid instances.
type SUPI struct {
	supiType supiType
	value    string // bare numeric IMSI (e.g. "001019756139935") or NAI string
}

// InvalidSUPI is a sentinel value representing an unset or invalid SUPI.
// It is the zero value of the SUPI type.
var InvalidSUPI SUPI = SUPI{}

const imsiPrefix = "imsi-"

// NewSUPIFromIMSI creates a SUPI from a bare numeric IMSI string.
// The IMSI must be 5 to 15 digits long and contain only digits.
func NewSUPIFromIMSI(imsi string) (SUPI, error) {
	if err := validateIMSI(imsi); err != nil {
		return InvalidSUPI, err
	}

	return SUPI{supiType: supiTypeIMSI, value: imsi}, nil
}

// NewSUPIFromPrefixed creates a SUPI from a string that may or may not
// have an "imsi-" prefix. Both "imsi-001019756139935" and "001019756139935"
// are accepted and normalized to the same internal representation.
func NewSUPIFromPrefixed(s string) (SUPI, error) {
	if len(s) >= len(imsiPrefix) && s[:len(imsiPrefix)] == imsiPrefix {
		return NewSUPIFromIMSI(s[len(imsiPrefix):])
	}

	return NewSUPIFromIMSI(s)
}

// NewSUPIFromNAI creates a SUPI from a Network Access Identifier.
// NAI-based SUPI is not yet supported; this constructor is a placeholder
// for future implementation.
func NewSUPIFromNAI(_ string) (SUPI, error) {
	return InvalidSUPI, fmt.Errorf("NAI SUPI not yet supported")
}

// String returns the prefixed string representation of the SUPI.
// For an IMSI-based SUPI, it returns "imsi-<digits>".
// For an NAI-based SUPI, it returns "nai-<value>".
// For an unset SUPI, it returns an empty string.
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

// IMSI returns the bare numeric IMSI digits (e.g. "001019756139935").
// The caller must ensure IsIMSI() is true before calling this method.
// It panics if the SUPI is unset or NAI-based.
func (s SUPI) IMSI() string {
	if s.supiType != supiTypeIMSI {
		panic("IMSI() called on non-IMSI SUPI")
	}

	return s.value
}

// IsValid returns true if the SUPI has been set to a valid value.
func (s SUPI) IsValid() bool {
	return s.supiType != supiTypeUnset
}

// IsIMSI returns true if the SUPI contains an IMSI.
func (s SUPI) IsIMSI() bool {
	return s.supiType == supiTypeIMSI
}

// IsNAI returns true if the SUPI contains an NAI.
func (s SUPI) IsNAI() bool {
	return s.supiType == supiTypeNAI
}

// validateIMSI checks that s is a valid IMSI: 5-15 digits, all numeric.
func validateIMSI(s string) error {
	if len(s) < 5 || len(s) > 15 {
		return fmt.Errorf("invalid IMSI length: %d (must be 5-15 digits)", len(s))
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
