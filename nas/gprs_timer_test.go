// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"testing"
	"time"
)

func TestGPRSTimer2FromDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want uint8
	}{
		{54 * time.Minute, 0x49}, // T3412 default: 9 decihours (010 01001)
		{30 * time.Minute, 0x3e}, // 30 minutes (001 11110)
		{10 * time.Second, 0x05}, // 5 × 2 seconds (000 00101)
		{6 * time.Minute, 0x26},  // 6 minutes in 1-minute units (001 00110)
	}

	for _, c := range cases {
		timer, err := GPRSTimer2FromDuration(c.d)
		if err != nil {
			t.Errorf("GPRSTimer2FromDuration(%v): unexpected error %v", c.d, err)
			continue
		}

		got, err := timer.MarshalBinary()
		if err != nil {
			t.Errorf("MarshalBinary(%v): %v", c.d, err)
			continue
		}

		if !bytes.Equal(got, []byte{c.want}) {
			t.Errorf("GPRSTimer2FromDuration(%v) = %#x, want %#x", c.d, got, c.want)
		}

		if d, ok := timer.Duration(); !ok || d != c.d {
			t.Errorf("Duration() = %v (%v), want %v", d, ok, c.d)
		}
	}
}

func TestGPRSTimer2Unrepresentable(t *testing.T) {
	// 100 minutes is not a whole number of 2 s / 1 min (≤31) / 6 min, so it has
	// no exact one-octet GPRS timer 2 encoding.
	if _, err := GPRSTimer2FromDuration(100 * time.Minute); err == nil {
		t.Fatal("expected an error for an unrepresentable duration")
	}
}

// TestGPRSTimer3NoSilentRounding confirms a duration the element cannot express
// exactly is rejected rather than rounded down to a shorter timer.
func TestGPRSTimer3NoSilentRounding(t *testing.T) {
	for _, d := range []time.Duration{45 * time.Second, 400 * time.Hour, 7 * time.Second} {
		if got, err := GPRSTimer3FromDuration(d); err == nil {
			t.Errorf("GPRSTimer3FromDuration(%v) = %+v, want an error", d, got)
		}
	}
}

func TestGPRSTimer3FromDuration(t *testing.T) {
	// The encoder picks the finest unit that represents the duration exactly, so
	// one hour becomes 6 × 10 minutes rather than 1 × 1 hour.
	cases := []struct {
		d    time.Duration
		want uint8
	}{
		{time.Hour, 0x06},        // 6 × 10 minutes (000 00110)
		{62 * time.Second, 0x7f}, // 31 × 2 seconds (011 11111)
		{30 * time.Second, 0x6f}, // 15 × 2 seconds (011 01111)
		{10 * time.Hour, 0x2a},   // 10 × 1 hour (001 01010)
		{320 * time.Hour, 0xc1},  // 1 × 320 hours (110 00001)
		{10 * time.Minute, 0x94}, // 20 × 30 seconds (100 10100)
		{310 * time.Hour, 0x5f},  // 31 × 10 hours (010 11111)
	}

	for _, c := range cases {
		timer, err := GPRSTimer3FromDuration(c.d)
		if err != nil {
			t.Errorf("GPRSTimer3FromDuration(%v): %v", c.d, err)
			continue
		}

		got, err := timer.MarshalBinary()
		if err != nil {
			t.Errorf("MarshalBinary(%v): %v", c.d, err)
			continue
		}

		if !bytes.Equal(got, []byte{c.want}) {
			t.Errorf("GPRSTimer3FromDuration(%v) = %#x, want %#x", c.d, got, c.want)
		}

		if d, ok := timer.Duration(); !ok || d != c.d {
			t.Errorf("Duration() = %v (%v), want %v", d, ok, c.d)
		}
	}
}

// TestGPRSTimerDeactivated confirms the deactivated unit is distinguishable from
// a zero-duration timer, in both directions.
func TestGPRSTimerDeactivated(t *testing.T) {
	off, err := ParseGPRSTimer3([]byte{0xE0}) // unit 111, value 0
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !off.Deactivated() {
		t.Fatal("unit 0b111 must decode as deactivated")
	}

	if _, ok := off.Duration(); ok {
		t.Fatal("a deactivated timer has no duration")
	}

	zero, err := ParseGPRSTimer3([]byte{0x60}) // unit 011 (2 s), value 0
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if zero.Deactivated() {
		t.Fatal("a zero-duration timer is not deactivated")
	}

	if d, ok := zero.Duration(); !ok || d != 0 {
		t.Fatalf("Duration() = %v (%v), want 0", d, ok)
	}

	// The deactivated encoding survives a round-trip.
	raw, err := off.MarshalBinary()
	if err != nil || !bytes.Equal(raw, []byte{0xE0}) {
		t.Fatalf("MarshalBinary = %#x err %v, want e0", raw, err)
	}
}

// TestGPRSTimerRoundTrip confirms every one-octet encoding decodes and re-encodes
// byte-for-byte, including reserved units.
func TestGPRSTimerRoundTrip(t *testing.T) {
	for i := range 256 {
		raw := []byte{uint8(i)}

		t2, err := ParseGPRSTimer2(raw)
		if err != nil {
			t.Fatalf("ParseGPRSTimer2(%#x): %v", raw, err)
		}

		got, err := t2.MarshalBinary()
		if err != nil || !bytes.Equal(got, raw) {
			t.Fatalf("GPRSTimer2 round-trip %#x -> %#x err %v", raw, got, err)
		}

		t3, err := ParseGPRSTimer3(raw)
		if err != nil {
			t.Fatalf("ParseGPRSTimer3(%#x): %v", raw, err)
		}

		got, err = t3.MarshalBinary()
		if err != nil || !bytes.Equal(got, raw) {
			t.Fatalf("GPRSTimer3 round-trip %#x -> %#x err %v", raw, got, err)
		}
	}
}
