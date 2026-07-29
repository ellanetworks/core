// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"
	"time"
)

// TestTimeZoneKnownValues checks the swapped-BCD-with-sign coding against
// offsets whose octets are well known (TS 23.040 §9.2.3.11).
func TestTimeZoneKnownValues(t *testing.T) {
	cases := map[byte]time.Duration{
		0x00: 0,
		0x40: 1 * time.Hour,                 // CET: 4 quarters, digits "04"
		0x80: 2 * time.Hour,                 // CEST: 8 quarters, digits "08"
		0x0A: -5 * time.Hour,                // US Eastern: 20 quarters, digits "20", negative
		0x23: 8 * time.Hour,                 // 32 quarters, digits "32"
		0x95: 14*time.Hour + 45*time.Minute, // 59 quarters, digits "59"
	}

	for octet, want := range cases {
		got, ok := TimeZone(octet).Offset()
		if !ok || got != want {
			t.Errorf("TimeZone(%#02x).Offset() = %s (ok %v), want %s", octet, got, ok, want)
		}

		back, err := NewTimeZone(want)
		if err != nil || byte(back) != octet {
			t.Errorf("NewTimeZone(%s) = %#02x (err %v), want %#02x", want, byte(back), err, octet)
		}
	}
}

// TestTimeZoneRoundTripsEveryOctet checks the element re-encodes unchanged
// whatever arrives, including the values whose nibbles are not decimal.
func TestTimeZoneRoundTripsEveryOctet(t *testing.T) {
	for i := range 256 {
		in := []byte{byte(i)}

		tz, err := ParseTimeZone(in)
		if err != nil {
			t.Fatalf("ParseTimeZone(%#02x): %v", i, err)
		}

		out, err := tz.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary(%#02x): %v", i, err)
		}

		if len(out) != 1 || out[0] != byte(i) {
			t.Fatalf("round-trip %#02x -> % x", i, out)
		}
	}
}

func TestTimeZoneAndTimeRoundTrip(t *testing.T) {
	when := time.Date(2026, time.July, 28, 14, 30, 5, 0, time.FixedZone("", 2*3600))

	in, err := NewTimeZoneAndTime(when)
	if err != nil {
		t.Fatalf("NewTimeZoneAndTime: %v", err)
	}

	raw, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	out, err := ParseTimeZoneAndTime(raw)
	if err != nil || out != in {
		t.Fatalf("round-trip = %+v (err %v), want %+v", out, err, in)
	}

	got, ok := out.Time()
	if !ok || !got.Equal(when) {
		t.Fatalf("Time() = %s (ok %v), want %s", got, ok, when)
	}
}

// TestTimeZoneAndTimeRejectsImpossibleDate checks a field that is decimal but
// not a real date is reported rather than silently rolled over.
func TestTimeZoneAndTimeRejectsImpossibleDate(t *testing.T) {
	// Month 13, which time.Date would roll into the next year.
	in := TimeZoneAndTime{Year: swapBCDPair(26), Month: swapBCDPair(13), Day: swapBCDPair(1)}

	if _, ok := in.Time(); ok {
		t.Fatal("month 13 must not decode to an instant")
	}
}

func TestDaylightSavingTime(t *testing.T) {
	for value, want := range map[DaylightSavingTime]time.Duration{
		DaylightSavingNone: 0, DaylightSavingOneHour: time.Hour, DaylightSavingTwoHour: 2 * time.Hour,
	} {
		if got, ok := value.Adjustment(); !ok || got != want {
			t.Errorf("Adjustment(%d) = %s (ok %v), want %s", value, got, ok, want)
		}
	}

	if _, ok := DaylightSavingTime(3).Adjustment(); ok {
		t.Error("the reserved value must not decode to an adjustment")
	}
}
