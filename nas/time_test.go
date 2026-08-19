// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"testing"
	"time"
	// tzdata is embedded so the daylight saving tests read the same zone rules
	// wherever they run, including images that carry no zoneinfo.
	_ "time/tzdata"
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

// TestTimeZoneAndTimeEncodesUniversalTime pins the octets, because the two
// halves of the element are on different clocks and only the bytes show it:
// TS 24.008 §10.5.3.9 puts "the universal time at which this information
// element may have been sent" in the timestamp and "the offset between
// universal time and local time" in the trailing octet. A round trip through
// Time cannot catch the timestamp being written as local time instead, since
// that names the same instant.
func TestTimeZoneAndTimeEncodesUniversalTime(t *testing.T) {
	// 14:30:05 at +02:00 is 12:30:05 UTC, so the hour octet must read 12.
	when := time.Date(2026, time.July, 28, 14, 30, 5, 0, time.FixedZone("", 2*3600))

	in, err := NewTimeZoneAndTime(when)
	if err != nil {
		t.Fatalf("NewTimeZoneAndTime: %v", err)
	}

	got, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	// Year 26, month 07, day 28, hour 12, minute 30, second 05, +2h = 8 quarters.
	want := []byte{0x62, 0x70, 0x82, 0x21, 0x03, 0x50, 0x80}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoding = % x, want % x", got, want)
	}
}

// TestTimeZoneAndTimeDecodesUniversalTime is the receiving half: the timestamp
// octets name a UTC instant, which the local offset then presents.
func TestTimeZoneAndTimeDecodesUniversalTime(t *testing.T) {
	parsed, err := ParseTimeZoneAndTime([]byte{0x62, 0x70, 0x82, 0x21, 0x03, 0x50, 0x80})
	if err != nil {
		t.Fatalf("ParseTimeZoneAndTime: %v", err)
	}

	got, ok := parsed.Time()
	if !ok {
		t.Fatal("Time() rejected a well-formed element")
	}

	if want := time.Date(2026, time.July, 28, 12, 30, 5, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("Time() = %s, want %s", got.UTC(), want)
	}

	// The offset it is presented at is the local time the network named.
	if _, offset := got.Zone(); offset != 2*3600 {
		t.Fatalf("Time() offset = %ds, want %ds", offset, 2*3600)
	}

	if h := got.Hour(); h != 14 {
		t.Fatalf("local hour = %d, want 14", h)
	}
}

// TestTimeZoneAndTimeUTCArgument checks an instant handed over already in UTC
// encodes as UTC at a zero offset rather than being shifted twice.
func TestTimeZoneAndTimeUTCArgument(t *testing.T) {
	when := time.Date(2026, time.July, 28, 12, 30, 5, 0, time.UTC)

	in, err := NewTimeZoneAndTime(when)
	if err != nil {
		t.Fatalf("NewTimeZoneAndTime: %v", err)
	}

	got, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	want := []byte{0x62, 0x70, 0x82, 0x21, 0x03, 0x50, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoding = % x, want % x", got, want)
	}
}

// TestNewDaylightSavingTime checks the adjustment is derived from the zone's
// own rules, in both hemispheres and across the summer-time boundary.
func TestNewDaylightSavingTime(t *testing.T) {
	cases := []struct {
		name string
		zone string
		when time.Time
		want DaylightSavingTime
	}{
		// Northern hemisphere: CET in winter, CEST in summer.
		{"europe winter", "Europe/Paris", time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC), DaylightSavingNone},
		{"europe summer", "Europe/Paris", time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC), DaylightSavingOneHour},
		// Southern hemisphere: the seasons, and so the samples, are reversed.
		{"sydney winter", "Australia/Sydney", time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC), DaylightSavingNone},
		{"sydney summer", "Australia/Sydney", time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC), DaylightSavingOneHour},
		// A zone that never adjusts must never claim an adjustment.
		{"no summer time", "Asia/Tokyo", time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC), DaylightSavingNone},
		{"utc", "UTC", time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC), DaylightSavingNone},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc, err := time.LoadLocation(c.zone)
			if err != nil {
				t.Fatalf("LoadLocation(%q): %v", c.zone, err)
			}

			if got := NewDaylightSavingTime(c.when.In(loc)); got != c.want {
				t.Fatalf("NewDaylightSavingTime = %s, want %s", got, c.want)
			}
		})
	}
}

// TestNewDaylightSavingTimeFixedZone checks a zone with no rules at all, which
// is what a network configured by bare offset presents.
func TestNewDaylightSavingTimeFixedZone(t *testing.T) {
	when := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.FixedZone("", 5*3600))

	if got := NewDaylightSavingTime(when); got != DaylightSavingNone {
		t.Fatalf("NewDaylightSavingTime = %s, want %s", got, DaylightSavingNone)
	}
}
