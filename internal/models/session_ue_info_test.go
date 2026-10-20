// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

func TestLocalTimeZoneIsAdjustedForDaylightSaving(t *testing.T) {
	cases := []struct {
		zone string
		when time.Time
		want string
	}{
		{"America/Montreal", time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC), "-05:00"},
		{"America/Montreal", time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC), "-04:00+1"},
		{"Asia/Kolkata", time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC), "+05:30"},
		{"Australia/Lord_Howe", time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC), "+11:00"},
		{"UTC", time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC), "+00:00"},
	}

	for _, tc := range cases {
		loc, err := time.LoadLocation(tc.zone)
		if err != nil {
			t.Skipf("time zone database lacks %s: %v", tc.zone, err)
		}

		if got := models.LocalTimeZone(tc.when.In(loc)); got != tc.want {
			t.Errorf("%s at %s = %q, want %q", tc.zone, tc.when.Format(time.DateOnly), got, tc.want)
		}
	}
}
