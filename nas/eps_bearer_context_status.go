// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "fmt"

// EPSBearerContextStatus is the EPS bearer context status information element
// value (TS 24.301 §9.9.2.1), which TS 24.501 §9.11.3.23A carries by reference:
// which of a UE's EPS bearer contexts are not in state BEARER CONTEXT-INACTIVE.
//
// Active is indexed by EPS bearer identity. Index 0 names no bearer — bit 1 of
// octet 3 is spare and coded as zero — so it is always false and is ignored on
// encode.
type EPSBearerContextStatus struct {
	Active [16]bool
}

// epsBearerContextStatusLen is the element's value length. TS 24.301 §9.9.2.1
// fixes the whole IE at four octets, so the value is exactly two.
const epsBearerContextStatusLen = 2

// ParseEPSBearerContextStatus decodes an EPS bearer context status IE value.
func ParseEPSBearerContextStatus(b []byte) (EPSBearerContextStatus, error) {
	if len(b) != epsBearerContextStatusLen {
		return EPSBearerContextStatus{}, fmt.Errorf(
			"nas: EPS bearer context status is %d octets, want %d", len(b), epsBearerContextStatusLen)
	}

	var out EPSBearerContextStatus

	// EBI(n) is bit (n mod 8) of octet (n div 8); EBI(0) is spare.
	for ebi := 1; ebi < 16; ebi++ {
		if b[ebi/8]&(1<<(ebi%8)) != 0 {
			out.Active[ebi] = true
		}
	}

	return out, nil
}

// AppendBinary encodes the EPS bearer context status IE value onto b.
func (s EPSBearerContextStatus) AppendBinary(b []byte) ([]byte, error) {
	var buf [epsBearerContextStatusLen]byte

	for ebi := 1; ebi < 16; ebi++ {
		if s.Active[ebi] {
			buf[ebi/8] |= 1 << (ebi % 8)
		}
	}

	return append(b, buf[:]...), nil
}

// MarshalBinary encodes the EPS bearer context status IE value.
func (s EPSBearerContextStatus) MarshalBinary() ([]byte, error) { return s.AppendBinary(nil) }

// Any reports whether any bearer context is active.
func (s EPSBearerContextStatus) Any() bool {
	for ebi := 1; ebi < 16; ebi++ {
		if s.Active[ebi] {
			return true
		}
	}

	return false
}

func (s EPSBearerContextStatus) String() string {
	out := make([]byte, 0, 32)

	for ebi := 1; ebi < 16; ebi++ {
		if !s.Active[ebi] {
			continue
		}

		if len(out) > 0 {
			out = append(out, ',')
		}

		out = fmt.Appendf(out, "%d", ebi)
	}

	if len(out) == 0 {
		return "none active"
	}

	return string(out)
}
