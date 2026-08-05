// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// bitRateUnits are the suffixes 3GPP TS 29.571 §5.4.4.16 allows, largest last
// so String can pick the widest unit that divides evenly.
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

// BitRate is a rate in bits per second. The field is unexported so the only way
// to obtain one is ParseBitRate or BitRateFromBps: a BitRate in hand has always
// been through a parser, which is what stops each consumer writing its own.
type BitRate struct {
	bps uint64
}

// BitRateFromBps builds a rate from a value already in bits per second.
func BitRateFromBps(bps uint64) BitRate {
	return BitRate{bps: bps}
}

// ParseBitRate converts an AMBR of the form "1 Gbps" (TS 29.571 §5.4.4.16).
// A malformed value is an error rather than a silent zero: a rate that quietly
// becomes 0 is sent on the wire as a limit the UE cannot exceed.
func ParseBitRate(s string) (BitRate, error) {
	value, unit, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok {
		return BitRate{}, fmt.Errorf("bit rate %q has no unit", s)
	}

	multiplier := uint64(0)

	for _, u := range bitRateUnits {
		if u.suffix == unit {
			multiplier = u.multiplier

			break
		}
	}

	if multiplier == 0 {
		return BitRate{}, fmt.Errorf("bit rate %q has unknown unit %q", s, unit)
	}

	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return BitRate{}, fmt.Errorf("bit rate %q: %w", s, err)
	}

	if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
		return BitRate{}, fmt.Errorf("bit rate %q is not a usable rate", s)
	}

	scaled := n * float64(multiplier)
	if scaled > math.MaxUint64 {
		return BitRate{}, fmt.Errorf("bit rate %q overflows", s)
	}

	return BitRate{bps: uint64(scaled)}, nil
}

// Bps returns the rate in bits per second.
func (b BitRate) Bps() uint64 { return b.bps }

// Kbps returns the rate in kilobits per second, truncated. EPS scales the
// APN-AMBR in kbps (TS 24.008 §10.5.6.5).
func (b BitRate) Kbps() uint64 { return b.bps / 1000 }

// IsZero reports whether the rate is unset.
func (b BitRate) IsZero() bool { return b.bps == 0 }

// String renders the widest unit that divides the rate evenly, so a parsed
// value round-trips without loss.
func (b BitRate) String() string {
	unit := bitRateUnits[0]

	// Zero divides evenly by every unit, so it would otherwise widen all the
	// way to "0 Tbps".
	for _, u := range bitRateUnits {
		if b.bps != 0 && b.bps%u.multiplier == 0 {
			unit = u
		}
	}

	return strconv.FormatUint(b.bps/unit.multiplier, 10) + " " + unit.suffix
}

func (b BitRate) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}

func (b *BitRate) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsed, err := ParseBitRate(s)
	if err != nil {
		return err
	}

	*b = parsed

	return nil
}

// MustParseBitRate is ParseBitRate for literals known good at author time, in
// the manner of netip.MustParseAddr. It panics, so it belongs in tests and
// package-level constants, never on a path handling configured input.
func MustParseBitRate(s string) BitRate {
	b, err := ParseBitRate(s)
	if err != nil {
		panic(err)
	}

	return b
}
