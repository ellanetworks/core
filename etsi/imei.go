package etsi

import (
	"fmt"
	"strings"
)

// IMEIFromPEI converts a 3GPP PEI string (as produced by the NAS layer) into
// a standard 15-digit IMEI suitable for device lookups.
//
// Accepted input formats:
//
//	"imei-353456789012345"   → already a 15-digit IMEI, returned as-is
//	"imeisv-3534567890123401" → 16-digit IMEISV, last 2 SV digits stripped,
//	                            Luhn check digit appended
//	"353456789012345"        → bare 15-digit IMEI (no prefix)
//	"3534567890123401"       → bare 16-digit IMEISV
//
// Returns an empty string if the input is empty, and an error if the format
// is unrecognizable.
func IMEIFromPEI(pei string) (string, error) {
	if pei == "" {
		return "", nil
	}

	digits := pei

	if strings.HasPrefix(pei, "imeisv-") {
		digits = pei[len("imeisv-"):]
	} else if strings.HasPrefix(pei, "imei-") {
		digits = pei[len("imei-"):]
	}

	if !isAllDigits(digits) {
		return "", fmt.Errorf("PEI contains non-digit characters: %s", pei)
	}

	switch len(digits) {
	case 15:
		// Already a standard IMEI (14 digits + check digit).
		return digits, nil
	case 16:
		// IMEISV: first 14 digits are TAC+serial, last 2 are SV.
		// Drop the SV and compute the Luhn check digit.
		base := digits[:14]
		return base + luhnCheckDigit(base), nil
	default:
		return "", fmt.Errorf("unexpected PEI digit length %d: %s", len(digits), pei)
	}
}

// luhnCheckDigit computes the Luhn check digit for a numeric string.
// The input must contain only ASCII digits. The returned digit, when
// appended, makes the full string pass the Luhn validation.
func luhnCheckDigit(digits string) string {
	sum := 0
	// Walking from the rightmost digit of the input. The check digit
	// (not yet present) would be position 1 (odd → not doubled), so
	// the rightmost payload digit is position 2 (even → doubled).
	double := true
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	check := (10 - (sum % 10)) % 10
	return string(rune('0' + check))
}
