// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "fmt"

// PLMNList is the PLMN list information element value (TS 24.008 §10.5.1.13),
// which TS 24.501 §9.11.3.45 and TS 24.301 §9.9.2.8 both carry as the equivalent
// PLMNs a UE may select: one 3-octet identity per entry.
type PLMNList []PLMN

// maxPLMNListEntries is the largest number of identities the element holds: 45
// value octets, three per identity (TS 24.008 §10.5.1.13).
const maxPLMNListEntries = 15

// ParsePLMNList decodes a PLMN list IE value.
func ParsePLMNList(v []byte) (PLMNList, error) {
	if len(v) == 0 || len(v)%3 != 0 {
		return nil, fmt.Errorf("nas: PLMN list is %d octets, want a non-zero multiple of 3", len(v))
	}

	if n := len(v) / 3; n > maxPLMNListEntries {
		return nil, fmt.Errorf("nas: PLMN list holds %d identities, the element carries %d", n, maxPLMNListEntries)
	}

	out := make(PLMNList, 0, len(v)/3)

	for i := 0; i < len(v); i += 3 {
		p, err := ParsePLMN([3]byte{v[i], v[i+1], v[i+2]})
		if err != nil {
			return nil, err
		}

		out = append(out, p)
	}

	return out, nil
}

// AppendBinary encodes the PLMN list IE value onto b.
func (l PLMNList) AppendBinary(b []byte) ([]byte, error) {
	if len(l) == 0 {
		return b, fmt.Errorf("nas: PLMN list carries no identity")
	}

	if len(l) > maxPLMNListEntries {
		return b, fmt.Errorf("nas: %d identities exceed the %d the element holds", len(l), maxPLMNListEntries)
	}

	out := b

	for _, p := range l {
		var err error
		if out, err = p.AppendBinary(out); err != nil {
			return b, err
		}
	}

	return out, nil
}

// MarshalBinary encodes the PLMN list IE value.
func (l PLMNList) MarshalBinary() ([]byte, error) { return l.AppendBinary(nil) }
