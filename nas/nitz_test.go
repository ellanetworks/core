// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"
	"time"
	// tzdata is embedded so the zone rules are the same wherever the test runs.
	_ "time/tzdata"
)

// TestNewNetworkTime checks the three elements agree with each other: the
// standalone time zone must be the one embedded in the universal time, and the
// adjustment must be the zone's own.
func TestNewNetworkTime(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	// Midsummer in Paris is CEST, UTC+2, of which one hour is summer time.
	when := time.Date(2026, time.July, 28, 12, 30, 5, 0, time.UTC).In(loc)

	got, err := NewNetworkTime(when)
	if err != nil {
		t.Fatalf("NewNetworkTime: %v", err)
	}

	if got.LocalTimeZone != got.UniversalTime.Zone {
		t.Errorf("local time zone %#02x disagrees with the universal time's %#02x",
			byte(got.LocalTimeZone), byte(got.UniversalTime.Zone))
	}

	offset, ok := got.LocalTimeZone.Offset()
	if !ok || offset != 2*time.Hour {
		t.Errorf("offset = %s (ok %v), want %s", offset, ok, 2*time.Hour)
	}

	if got.DaylightSavingTime != DaylightSavingOneHour {
		t.Errorf("daylight saving = %s, want %s", got.DaylightSavingTime, DaylightSavingOneHour)
	}

	// The timestamp is universal time, not the Paris wall clock.
	instant, ok := got.UniversalTime.Time()
	if !ok || !instant.Equal(when) {
		t.Errorf("universal time = %s (ok %v), want %s", instant, ok, when)
	}

	if h := instant.UTC().Hour(); h != 12 {
		t.Errorf("universal hour = %d, want 12", h)
	}
}

// TestNewNetworkTimeWinter checks the adjustment is reported as none out of
// season, so a UE is never told to add an hour twice.
func TestNewNetworkTimeWinter(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	got, err := NewNetworkTime(time.Date(2026, time.January, 28, 12, 30, 5, 0, time.UTC).In(loc))
	if err != nil {
		t.Fatalf("NewNetworkTime: %v", err)
	}

	offset, ok := got.LocalTimeZone.Offset()
	if !ok || offset != time.Hour {
		t.Errorf("offset = %s (ok %v), want %s", offset, ok, time.Hour)
	}

	if got.DaylightSavingTime != DaylightSavingNone {
		t.Errorf("daylight saving = %s, want %s", got.DaylightSavingTime, DaylightSavingNone)
	}
}

// TestNewNetworkTimeRejectsUnrepresentableYear checks the error reaches the
// caller rather than a half-filled set being sent.
func TestNewNetworkTimeRejectsUnrepresentableYear(t *testing.T) {
	if _, err := NewNetworkTime(time.Date(1999, time.December, 31, 23, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("a year the element cannot hold must be reported")
	}
}
