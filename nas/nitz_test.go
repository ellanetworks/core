// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"
	"time"
	_ "time/tzdata"
)

// TS 24.501 §3.1
func TestNewNetworkTime(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

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

	instant, ok := got.UniversalTime.Time()
	if !ok || !instant.Equal(when) {
		t.Errorf("universal time = %s (ok %v), want %s", instant, ok, when)
	}

	if h := instant.UTC().Hour(); h != 12 {
		t.Errorf("universal hour = %d, want 12", h)
	}
}

// TS 24.008 §10.5.3.12
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

// TS 24.008 §10.5.3.9
func TestNewNetworkTimeRejectsUnrepresentableYear(t *testing.T) {
	if _, err := NewNetworkTime(time.Date(1999, time.December, 31, 23, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("a year the element cannot hold must be reported")
	}
}
