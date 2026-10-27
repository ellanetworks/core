// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"math"
	"testing"

	"github.com/ellanetworks/core/diameter"
)

func TestBandwidthRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		b        Bandwidth
		extended bool
	}{
		{0, false},
		{1, false},
		{math.MaxUint32, false},
		{4294968000, true},
		{6_000_000_000, true},
		{math.MaxUint32 * bitsPerKilobit, true},
	} {
		avps, err := MaxRequestedBandwidthUL.Append(nil, tt.b)
		if err != nil {
			t.Fatalf("%d: %v", tt.b, err)
		}

		if _, ok := diameter.Find(avps, AVPExtendedMaxRequestedBWUL, VendorID); ok != tt.extended {
			t.Errorf("%d: extended = %v", tt.b, ok)
		}

		if bps, _ := diameter.Find(avps, AVPMaxRequestedBandwidthUL, VendorID); bps.Flags&diameter.AVPFlagMandatory == 0 {
			t.Errorf("%d: Max-Requested-Bandwidth-UL without M", tt.b)
		}

		got, err := MaxRequestedBandwidthUL.Parse(avps)
		if err != nil || got == nil || *got != tt.b {
			t.Errorf("%d parses as %v, %v", tt.b, got, err)
		}
	}
}

func TestBandwidthUnrepresentable(t *testing.T) {
	for _, b := range []Bandwidth{1 << 32, 4294967999, 6_000_000_001, (math.MaxUint32 + 1) * bitsPerKilobit} {
		if b.Representable() {
			t.Errorf("%d representable", b)
		}

		if _, err := MaxRequestedBandwidthDL.Append(nil, b); !errors.Is(err, ErrInvalidBandwidth) {
			t.Errorf("%d: err = %v", b, err)
		}
	}
}

func TestParseBandwidth(t *testing.T) {
	u32 := func(code, v uint32) diameter.AVP { return diameter.Unsigned32(code, 0, VendorID, v) }

	if got, err := MaxRequestedBandwidthDL.Parse(nil); got != nil || err != nil {
		t.Fatalf("absent = %v, %v", got, err)
	}

	got, err := MaxRequestedBandwidthDL.Parse([]diameter.AVP{u32(AVPMaxRequestedBandwidthDL, 7), u32(AVPExtendedMaxRequestedBWDL, 5_000_000)})
	if err != nil || *got != 5_000_000_000 {
		t.Fatalf("extended = %v, %v", got, err)
	}

	_, err = MaxRequestedBandwidthDL.Parse([]diameter.AVP{diameter.OctetString(AVPMaxRequestedBandwidthDL, 0, VendorID, []byte{1})})

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) || avpErr.ResultCode != diameter.ResultInvalidAVPLength {
		t.Fatalf("short = %v", err)
	}
}
