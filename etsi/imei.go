// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package etsi

import (
	"fmt"
	"strconv"
	"strings"
)

// IMEI is a mobile-equipment identity in IMEI (15-digit) or IMEISV (16-digit)
// format. It is the identity carried by the 4G IMEISV mobile identity
// (TS 24.301) and by the 5G PEI — TS 23.501 §5.9.3: "a PEI in the IMEI or
// IMEISV format" — so the MME's IMEI and the AMF's PEI store the same type. The
// zero value is unset.
type IMEI struct {
	digits string // 15 (IMEI) or 16 (IMEISV) decimal digits, no prefix; "" = unset
}

// NewIMEIFromPEI parses a 3GPP PEI/IMEISV string as produced by the NAS layer:
//
//	"imei-353456789012345"    → 15-digit IMEI
//	"imeisv-3534567890123401" → 16-digit IMEISV
//	"353456789012345"         → bare 15-digit IMEI
//	"3534567890123401"        → bare 16-digit IMEISV
//
// An empty input yields the unset zero value with no error; a non-digit or
// wrong-length payload is an error.
func NewIMEIFromPEI(pei string) (IMEI, error) {
	if pei == "" {
		return IMEI{}, nil
	}

	digits := pei

	switch {
	case strings.HasPrefix(pei, "imeisv-"):
		digits = pei[len("imeisv-"):]
	case strings.HasPrefix(pei, "imei-"):
		digits = pei[len("imei-"):]
	}

	if !isAllDigits(digits) {
		return IMEI{}, fmt.Errorf("PEI contains non-digit characters: %s", pei)
	}

	if l := len(digits); l != 15 && l != 16 {
		return IMEI{}, fmt.Errorf("unexpected PEI digit length %d: %s", l, pei)
	}

	return IMEI{digits: digits}, nil
}

func (e IMEI) IsSet() bool { return e.digits != "" }

// IMEI returns the normalized 15-digit IMEI (14-digit TAC+serial plus Luhn
// check digit) suitable for device lookups. An IMEISV's 2-digit software
// version is dropped and the check digit recomputed. Empty when unset.
func (e IMEI) IMEI() string {
	switch len(e.digits) {
	case 15:
		return e.digits
	case 16:
		base := e.digits[:14]
		return base + luhnCheckDigit(base)
	default:
		return ""
	}
}

// String returns the identity in its NAS-prefixed form ("imei-…" / "imeisv-…"),
// preserving the software version, or "" when unset. Use IMEI() for the
// normalized 15-digit device identity.
func (e IMEI) String() string {
	switch len(e.digits) {
	case 15:
		return "imei-" + e.digits
	case 16:
		return "imeisv-" + e.digits
	default:
		return ""
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

	return strconv.Itoa(check)
}
