// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestSMSSettingsDefaultsToDisabled(t *testing.T) {
	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	defer func() { _ = database.Close() }()

	settings, err := database.GetSMSSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSMSSettings: %s", err)
	}

	if *settings != db.DefaultSMSSettings() {
		t.Fatalf("settings = %+v, want %+v", *settings, db.DefaultSMSSettings())
	}

	if settings.Enabled {
		t.Fatal("SMS enabled on a fresh database")
	}
}

func TestSMSSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	defer func() { _ = database.Close() }()

	want := db.SMSSettings{Enabled: true, SMSCAddress: "10.0.0.5", SMSCPort: 3869, SMSNumber: "15550001111"}

	if err := database.UpdateSMSSettings(ctx, &want); err != nil {
		t.Fatalf("UpdateSMSSettings: %s", err)
	}

	got, err := database.GetSMSSettings(ctx)
	if err != nil {
		t.Fatalf("GetSMSSettings: %s", err)
	}

	if *got != want {
		t.Fatalf("settings = %+v, want %+v enabled", *got, want)
	}

	disabled := want
	disabled.Enabled = false

	if err := database.UpdateSMSSettings(ctx, &disabled); err != nil {
		t.Fatalf("UpdateSMSSettings disabling: %s", err)
	}

	got, err = database.GetSMSSettings(ctx)
	if err != nil {
		t.Fatalf("GetSMSSettings: %s", err)
	}

	if *got != disabled {
		t.Fatalf("settings = %+v, want %+v disabled with its settings kept", *got, disabled)
	}
}

func TestSMSSettingsValidate(t *testing.T) {
	cases := []struct {
		name     string
		settings db.SMSSettings
		valid    bool
	}{
		{"disabled", db.DefaultSMSSettings(), true},
		{"ipv4", db.SMSSettings{Enabled: true, SMSCAddress: "192.0.2.1", SMSCPort: 3868, SMSNumber: "15550001111"}, true},
		{"ipv6", db.SMSSettings{Enabled: true, SMSCAddress: "2001:db8::1", SMSCPort: 3868, SMSNumber: "15550001111"}, true},
		{"hostname", db.SMSSettings{Enabled: true, SMSCAddress: "smsc.example.org", SMSCPort: 3868, SMSNumber: "15550001111"}, false},
		{"unspecified", db.SMSSettings{Enabled: true, SMSCAddress: "0.0.0.0", SMSCPort: 3868, SMSNumber: "15550001111"}, false},
		{"zone", db.SMSSettings{Enabled: true, SMSCAddress: "fe80::1%eth0", SMSCPort: 3868, SMSNumber: "15550001111"}, false},
		{"port zero", db.SMSSettings{Enabled: true, SMSCAddress: "192.0.2.1", SMSCPort: 0, SMSNumber: "15550001111"}, false},
		{"port too high", db.SMSSettings{Enabled: true, SMSCAddress: "192.0.2.1", SMSCPort: 65536, SMSNumber: "15550001111"}, false},
		{"enabled without a number", db.SMSSettings{Enabled: true, SMSCAddress: "192.0.2.1", SMSCPort: 3868}, false},
		{"enabled without an address", db.SMSSettings{Enabled: true, SMSCPort: 3868, SMSNumber: "15550001111"}, false},
		{"plus sign", db.SMSSettings{Enabled: true, SMSCAddress: "192.0.2.1", SMSCPort: 3868, SMSNumber: "+15550001111"}, false},
		{"disabled without a number", db.SMSSettings{SMSCAddress: "192.0.2.1", SMSCPort: 3868}, true},
		{"disabled with an invalid address", db.SMSSettings{SMSCAddress: "smsc.example.org", SMSCPort: 3868}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.settings.Validate()
			if (err == nil) != tc.valid {
				t.Fatalf("Validate(%+v) = %v, want valid=%v", tc.settings, err, tc.valid)
			}
		})
	}
}
