// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"fmt"
)

var ErrInvalidPLMN = errors.New("tgpp: invalid PLMN")

const (
	plmnLen    = 3
	plmnFiller = 0xf
)

type PLMN struct {
	MCC string
	MNC string
}

func (p PLMN) Valid() bool {
	return len(p.MCC) == 3 && (len(p.MNC) == 2 || len(p.MNC) == 3) && decimal(p.MCC) && decimal(p.MNC)
}

func (p PLMN) String() string {
	return p.MCC + p.MNC
}

func (p PLMN) Octets() ([plmnLen]byte, error) {
	if !p.Valid() {
		return [plmnLen]byte{}, fmt.Errorf("%w: MCC %q MNC %q", ErrInvalidPLMN, p.MCC, p.MNC)
	}

	mnc3 := byte(plmnFiller)
	if len(p.MNC) == 3 {
		mnc3 = p.MNC[2] - '0'
	}

	return [plmnLen]byte{
		(p.MCC[1]-'0')<<4 | (p.MCC[0] - '0'),
		mnc3<<4 | (p.MCC[2] - '0'),
		(p.MNC[1]-'0')<<4 | (p.MNC[0] - '0'),
	}, nil
}

func (p PLMN) AppendBinary(b []byte) ([]byte, error) {
	o, err := p.Octets()
	if err != nil {
		return b, err
	}

	return append(b, o[:]...), nil
}

func (p PLMN) MarshalBinary() ([]byte, error) { return p.AppendBinary(nil) }

func ParsePLMN(b []byte) (PLMN, error) {
	if len(b) != plmnLen {
		return PLMN{}, fmt.Errorf("%w: %d octets", ErrInvalidPLMN, len(b))
	}

	digits := []byte{b[0] & 0xf, b[0] >> 4, b[1] & 0xf, b[2] & 0xf, b[2] >> 4}
	if mnc3 := b[1] >> 4; mnc3 != plmnFiller {
		digits = append(digits, mnc3)
	}

	for i, d := range digits {
		if d > 9 {
			return PLMN{}, fmt.Errorf("%w: digit %d is %#x", ErrInvalidPLMN, i+1, d)
		}

		digits[i] = '0' + d
	}

	return PLMN{MCC: string(digits[:3]), MNC: string(digits[3:])}, nil
}

func ParseMCCMNC(s string) (PLMN, error) {
	if len(s) != 5 && len(s) != 6 {
		return PLMN{}, fmt.Errorf("%w: MCC-MNC %q", ErrInvalidPLMN, s)
	}

	p := PLMN{MCC: s[:3], MNC: s[3:]}
	if !p.Valid() {
		return PLMN{}, fmt.Errorf("%w: MCC-MNC %q", ErrInvalidPLMN, s)
	}

	return p, nil
}
