// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

const (
	peerA = "01890000-0000-7000-8000-00000000000a"
	peerB = "01890000-0000-7000-8000-00000000000b"
)

func newSMSDatabase(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

func TestSMSSettingsDefaultsToDisabled(t *testing.T) {
	database := newSMSDatabase(t)

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
	database := newSMSDatabase(t)

	if err := database.CreateSMSCPeer(ctx, &db.SMSCPeer{ID: peerA, Address: "10.0.0.5", Port: 3869, ServiceCentres: []string{"15550000000"}}); err != nil {
		t.Fatalf("CreateSMSCPeer: %s", err)
	}

	want := db.SMSSettings{Enabled: true, SMSNumber: "15550001111"}

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

func TestSMSCanBeEnabledBeforeItIsSetUp(t *testing.T) {
	ctx := context.Background()
	database := newSMSDatabase(t)

	if err := database.UpdateSMSSettings(ctx, &db.SMSSettings{Enabled: true}); err != nil {
		t.Fatalf("UpdateSMSSettings without a peer or a number: %s", err)
	}

	if err := database.CreateSMSCPeer(ctx, &db.SMSCPeer{ID: peerA, Address: "192.0.2.1", Port: 3868, ServiceCentres: []string{"15550000000"}}); err != nil {
		t.Fatalf("CreateSMSCPeer: %s", err)
	}

	if err := database.DeleteSMSCPeer(ctx, peerA); err != nil {
		t.Fatalf("DeleteSMSCPeer of the last peer while SMS is on: %s", err)
	}

	if err := database.DeleteSMSCPeer(ctx, peerA); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("DeleteSMSCPeer of a missing peer = %v, want ErrNotFound", err)
	}
}

func TestSMSCPeersRoundTrip(t *testing.T) {
	ctx := context.Background()
	database := newSMSDatabase(t)

	a := db.SMSCPeer{ID: peerA, Address: "192.0.2.1", Port: 3868, DiameterIdentity: "smsc-a.example.org", ServiceCentres: []string{"15550000001", "15550000000"}}
	b := db.SMSCPeer{ID: peerB, Address: "192.0.2.1", Port: 3869, DiameterIdentity: "smsc-b.example.org", ServiceCentres: []string{"15550000002"}}

	for _, p := range []db.SMSCPeer{a, b} {
		if err := database.CreateSMSCPeer(ctx, &p); err != nil {
			t.Fatalf("CreateSMSCPeer: %s", err)
		}
	}

	a.Port = 3870
	a.ServiceCentres = []string{"15550000003"}

	if err := database.UpdateSMSCPeer(ctx, &a); err != nil {
		t.Fatalf("UpdateSMSCPeer: %s", err)
	}

	peers, err := database.ListSMSCPeers(ctx)
	if err != nil {
		t.Fatalf("ListSMSCPeers: %s", err)
	}

	want := []db.SMSCPeer{a, b}
	if !slices.EqualFunc(peers, want, func(x, y db.SMSCPeer) bool {
		return x.ID == y.ID && x.Address == y.Address && x.Port == y.Port && x.DiameterIdentity == y.DiameterIdentity && slices.Equal(x.ServiceCentres, y.ServiceCentres)
	}) {
		t.Fatalf("peers = %+v, want %+v", peers, want)
	}

	got, err := database.GetSMSCPeer(ctx, peerB)
	if err != nil || got.Port != 3869 {
		t.Fatalf("GetSMSCPeer = %+v, %v, want peer B", got, err)
	}

	if _, err := database.GetSMSCPeer(ctx, "missing"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("GetSMSCPeer of a missing peer = %v, want ErrNotFound", err)
	}

	missing := b
	missing.ID = "missing"

	if err := database.UpdateSMSCPeer(ctx, &missing); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("UpdateSMSCPeer of a missing peer = %v, want ErrNotFound", err)
	}
}

func TestSMSCPeerConflictsAreRejected(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name  string
		first db.SMSCPeer
		next  db.SMSCPeer
	}{
		{
			"shared service centre",
			db.SMSCPeer{ID: peerA, Address: "192.0.2.1", Port: 3868, ServiceCentres: []string{"15550000000"}},
			db.SMSCPeer{ID: peerB, Address: "192.0.2.2", Port: 3868, ServiceCentres: []string{"15550000000"}},
		},
		{
			"shared address without identities",
			db.SMSCPeer{ID: peerA, Address: "192.0.2.1", Port: 3868, DiameterIdentity: "smsc-a.example.org", ServiceCentres: []string{"15550000000"}},
			db.SMSCPeer{ID: peerB, Address: "192.0.2.1", Port: 3869, ServiceCentres: []string{"15550000001"}},
		},
		{
			"shared identity",
			db.SMSCPeer{ID: peerA, Address: "192.0.2.1", Port: 3868, DiameterIdentity: "smsc.example.org", ServiceCentres: []string{"15550000000"}},
			db.SMSCPeer{ID: peerB, Address: "192.0.2.2", Port: 3868, DiameterIdentity: "SMSC.example.org", ServiceCentres: []string{"15550000001"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database := newSMSDatabase(t)

			if err := database.CreateSMSCPeer(ctx, &tc.first); err != nil {
				t.Fatalf("CreateSMSCPeer: %s", err)
			}

			if err := database.CreateSMSCPeer(ctx, &tc.next); !errors.Is(err, db.ErrSMSCPeerConflict) {
				t.Fatalf("CreateSMSCPeer = %v, want ErrSMSCPeerConflict", err)
			}

			peers, err := database.ListSMSCPeers(ctx)
			if err != nil || len(peers) != 1 {
				t.Fatalf("ListSMSCPeers = %+v, %v, want only the first peer", peers, err)
			}
		})
	}
}

func TestSMSSettingsValidate(t *testing.T) {
	cases := []struct {
		name     string
		settings db.SMSSettings
		valid    bool
	}{
		{"disabled", db.DefaultSMSSettings(), true},
		{"enabled", db.SMSSettings{Enabled: true, SMSNumber: "15550001111"}, true},
		{"enabled without a number", db.SMSSettings{Enabled: true}, true},
		{"plus sign", db.SMSSettings{Enabled: true, SMSNumber: "+15550001111"}, false},
		{"disabled with an invalid number", db.SMSSettings{SMSNumber: "0123"}, false},
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

func TestSMSCPeerValidate(t *testing.T) {
	valid := db.SMSCPeer{ID: peerA, Address: "192.0.2.1", Port: 3868, ServiceCentres: []string{"15550000000"}}

	cases := []struct {
		name  string
		edit  func(p *db.SMSCPeer)
		valid bool
	}{
		{"ipv4", func(*db.SMSCPeer) {}, true},
		{"ipv6", func(p *db.SMSCPeer) { p.Address = "2001:db8::1" }, true},
		{"identity", func(p *db.SMSCPeer) { p.DiameterIdentity = "smsc.example.org" }, true},
		{"identity with an underscore", func(p *db.SMSCPeer) { p.DiameterIdentity = "smsc_1.example.org" }, true},
		{"non-canonical address", func(p *db.SMSCPeer) { p.Address = "2001:DB8::1" }, false},
		{"hostname", func(p *db.SMSCPeer) { p.Address = "smsc.example.org" }, false},
		{"unspecified", func(p *db.SMSCPeer) { p.Address = "0.0.0.0" }, false},
		{"zone", func(p *db.SMSCPeer) { p.Address = "fe80::1%eth0" }, false},
		{"port zero", func(p *db.SMSCPeer) { p.Port = 0 }, false},
		{"port too high", func(p *db.SMSCPeer) { p.Port = 65536 }, false},
		{"single label identity", func(p *db.SMSCPeer) { p.DiameterIdentity = "smsc" }, false},
		{"identity with a space", func(p *db.SMSCPeer) { p.DiameterIdentity = "smsc .example.org" }, false},
		{"no service centre", func(p *db.SMSCPeer) { p.ServiceCentres = nil }, false},
		{"invalid service centre", func(p *db.SMSCPeer) { p.ServiceCentres = []string{"+15550000000"} }, false},
		{"duplicate service centre", func(p *db.SMSCPeer) { p.ServiceCentres = []string{"15550000000", "15550000000"} }, false},
		{"no id", func(p *db.SMSCPeer) { p.ID = "" }, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			p.ServiceCentres = slices.Clone(valid.ServiceCentres)
			tc.edit(&p)

			if err := p.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate(%+v) = %v, want valid=%v", p, err, tc.valid)
			}
		})
	}
}
