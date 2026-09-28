// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "fmt"

const laiLen = 5

// LAI is a location area identification (TS 24.008 §10.5.1.3).
type LAI struct {
	PLMN PLMN
	LAC  uint16
}

// String renders the identity as "mcc-mnc-lac".
func (l LAI) String() string { return fmt.Sprintf("%s-%04x", l.PLMN, l.LAC) }

// ParseLAI decodes a 5-octet location area identification value.
func ParseLAI(b []byte) (LAI, error) {
	if len(b) != laiLen {
		return LAI{}, fmt.Errorf("nas: location area identification is %d octets, want %d", len(b), laiLen)
	}

	plmn, err := ParsePLMN([3]byte(b[:3]))
	if err != nil {
		return LAI{}, err
	}

	return LAI{PLMN: plmn, LAC: uint16(b[3])<<8 | uint16(b[4])}, nil
}

// AppendBinary encodes the location area identification value onto b.
func (l LAI) AppendBinary(b []byte) ([]byte, error) {
	out, err := l.PLMN.AppendBinary(b)
	if err != nil {
		return b, err
	}

	return append(out, byte(l.LAC>>8), byte(l.LAC)), nil
}

// MarshalBinary encodes the location area identification value.
func (l LAI) MarshalBinary() ([]byte, error) { return l.AppendBinary(nil) }
