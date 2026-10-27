// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"fmt"
	"math/bits"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/models"
)

var bitRateUnits = []struct {
	suffix     string
	multiplier uint64
}{
	{"bps", 1},
	{"Kbps", 1e3},
	{"Mbps", 1e6},
	{"Gbps", 1e9},
	{"Tbps", 1e12},
}

const maxFractionDigits = 12

type BitRate string

func (b BitRate) Bps() (uint64, error) {
	s := string(b)

	value, unit, ok := strings.Cut(s, " ")
	if !ok {
		return 0, fmt.Errorf("wire: bit rate %q has no unit", s)
	}

	var multiplier uint64

	for _, u := range bitRateUnits {
		if u.suffix == unit {
			multiplier = u.multiplier
		}
	}

	if multiplier == 0 {
		return 0, fmt.Errorf("wire: bit rate %q has unknown unit", s)
	}

	whole, frac, hasFrac := strings.Cut(value, ".")
	if !allDigits(whole) || hasFrac && !allDigits(frac) {
		return 0, fmt.Errorf("wire: bit rate %q is not a decimal number", s)
	}

	w, err := strconv.ParseUint(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("wire: bit rate %q overflows", s)
	}

	hi, bps := bits.Mul64(w, multiplier)
	if hi != 0 {
		return 0, fmt.Errorf("wire: bit rate %q overflows", s)
	}

	total, carry := bits.Add64(bps, fractionBps(frac, multiplier), 0)
	if carry != 0 {
		return 0, fmt.Errorf("wire: bit rate %q overflows", s)
	}

	return total, nil
}

func fractionBps(frac string, multiplier uint64) uint64 {
	frac = strings.TrimRight(frac, "0")
	if frac == "" {
		return 0
	}

	digits, rest := frac, ""
	if len(digits) > maxFractionDigits {
		digits, rest = frac[:maxFractionDigits], frac[maxFractionDigits:]
	}

	n, _ := strconv.ParseUint(digits, 10, 64)

	scale := uint64(1)
	for range len(digits) {
		scale *= 10
	}

	hi, lo := bits.Mul64(n, multiplier)

	q, r := bits.Div64(hi, lo, scale)
	if r != 0 || rest != "" {
		q++
	}

	return q
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

func (b BitRate) Check(Policy) error {
	if b == "" {
		return Missing("")
	}

	if _, err := b.Bps(); err != nil {
		return Incorrect("", false, "%v", err)
	}

	return nil
}

func BitRateFromBps(bps uint64) BitRate {
	unit := bitRateUnits[0]

	for _, u := range bitRateUnits {
		if bps != 0 && bps%u.multiplier == 0 {
			unit = u
		}
	}

	return BitRate(strconv.FormatUint(bps/unit.multiplier, 10) + " " + unit.suffix)
}

func BitRateFromModel(m models.BitRate) BitRate {
	return BitRateFromBps(m.Bps())
}

func (b BitRate) Model() (models.BitRate, error) {
	bps, err := b.Bps()
	if err != nil {
		return models.BitRate{}, err
	}

	return models.BitRateFromBps(bps), nil
}

type Ambr struct {
	Uplink   BitRate `json:"uplink"`
	Downlink BitRate `json:"downlink"`
}

func (a Ambr) Check(p Policy) error {
	if err := a.Uplink.Check(p); err != nil {
		return Nest("/uplink", err)
	}

	if err := a.Downlink.Check(p); err != nil {
		return Nest("/downlink", err)
	}

	return nil
}

func AmbrFromModel(m models.Ambr) Ambr {
	return Ambr{Uplink: BitRateFromModel(m.Uplink), Downlink: BitRateFromModel(m.Downlink)}
}

func (a Ambr) Model() (models.Ambr, error) {
	ul, err := a.Uplink.Model()
	if err != nil {
		return models.Ambr{}, err
	}

	dl, err := a.Downlink.Model()
	if err != nil {
		return models.Ambr{}, err
	}

	return models.Ambr{Uplink: ul, Downlink: dl}, nil
}
