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
	smscA  = "01890000-0000-7000-8000-00000000000a"
	smscB  = "01890000-0000-7000-8000-00000000000b"
	nodeA1 = "01890000-0000-7000-8000-0000000000a1"
	nodeA2 = "01890000-0000-7000-8000-0000000000a2"
	nodeA3 = "01890000-0000-7000-8000-0000000000a3"
	nodeB1 = "01890000-0000-7000-8000-0000000000b1"
	nodeB2 = "01890000-0000-7000-8000-0000000000b2"
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

func TestSMSSettingsDefaults(t *testing.T) {
	database := newSMSDatabase(t)

	settings, err := database.GetSMSSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSMSSettings: %s", err)
	}

	if *settings != db.DefaultSMSSettings() {
		t.Fatalf("settings = %+v, want %+v", *settings, db.DefaultSMSSettings())
	}

	if settings.SMSNumber != "15550001111" {
		t.Fatalf("SMS number on a fresh database = %q, want 15550001111", settings.SMSNumber)
	}
}

func TestSMSSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	database := newSMSDatabase(t)

	if err := database.CreateSMSC(ctx, smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "smsc-10-0-0-5.example.org", "10.0.0.5", 3869))); err != nil {
		t.Fatalf("CreateSMSC: %s", err)
	}

	want := db.SMSSettings{SMSNumber: "15550009999"}

	if err := database.UpdateSMSSettings(ctx, &want); err != nil {
		t.Fatalf("UpdateSMSSettings: %s", err)
	}

	got, err := database.GetSMSSettings(ctx)
	if err != nil {
		t.Fatalf("GetSMSSettings: %s", err)
	}

	if *got != want {
		t.Fatalf("settings = %+v, want %+v", *got, want)
	}

	cleared := db.SMSSettings{}

	if err := database.UpdateSMSSettings(ctx, &cleared); err != nil {
		t.Fatalf("UpdateSMSSettings clearing the number: %s", err)
	}

	got, err = database.GetSMSSettings(ctx)
	if err != nil {
		t.Fatalf("GetSMSSettings: %s", err)
	}

	if *got != cleared {
		t.Fatalf("settings = %+v, want %+v", *got, cleared)
	}
}

func TestLastSMSCCanBeDeleted(t *testing.T) {
	ctx := context.Background()
	database := newSMSDatabase(t)

	if err := database.CreateSMSC(ctx, smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "smsc-192-0-2-1.example.org", "192.0.2.1", 3868))); err != nil {
		t.Fatalf("CreateSMSC: %s", err)
	}

	if err := database.DeleteSMSC(ctx, smscA); err != nil {
		t.Fatalf("DeleteSMSC of the last SMSC: %s", err)
	}

	if err := database.DeleteSMSC(ctx, smscA); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("DeleteSMSC of a missing SMSC = %v, want ErrNotFound", err)
	}

	if err := database.CreateSMSC(ctx, smsc(smscB, "b", []string{"15550000000"}, node(nodeA1, "smsc-192-0-2-1.example.org", "192.0.2.1", 3868))); err != nil {
		t.Fatalf("CreateSMSC reusing the deleted node and service centre: %s", err)
	}
}

func TestSMSCsRoundTrip(t *testing.T) {
	ctx := context.Background()
	database := newSMSDatabase(t)

	a := *smsc(smscA, "a", []string{"15550000001", "15550000000"},
		node(nodeA1, "smsc1.node.example.org", "192.0.2.1", 3868),
		node(nodeA2, "smsc2.node.example.org", "192.0.2.2", 3868),
	)
	b := *smsc(smscB, "b", []string{"15550000002"}, node(nodeB1, "smsc-b.example.org", "192.0.2.1", 3869))

	for _, s := range []db.SMSC{a, b} {
		if err := database.CreateSMSC(ctx, &s); err != nil {
			t.Fatalf("CreateSMSC: %s", err)
		}
	}

	a.ServiceCentres = []string{"15550000003"}
	a.Nodes = []db.SMSCNode{a.Nodes[1], *node(nodeA3, "smsc3.node.example.org", "192.0.2.3", 3870)}

	if err := database.UpdateSMSC(ctx, &a); err != nil {
		t.Fatalf("UpdateSMSC: %s", err)
	}

	smscs, err := database.ListSMSCs(ctx)
	if err != nil {
		t.Fatalf("ListSMSCs: %s", err)
	}

	want := []db.SMSC{a, b}
	if !slices.EqualFunc(smscs, want, sameSMSC) {
		t.Fatalf("SMSCs = %+v, want %+v", smscs, want)
	}

	got, err := database.GetSMSC(ctx, "b")
	if err != nil || !sameSMSC(*got, b) {
		t.Fatalf("GetSMSC = %+v, %v, want SMSC b", got, err)
	}

	if _, err := database.GetSMSC(ctx, "missing"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("GetSMSC of a missing SMSC = %v, want ErrNotFound", err)
	}

	missing := b
	missing.ID = "missing"

	if err := database.UpdateSMSC(ctx, &missing); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("UpdateSMSC of a missing SMSC = %v, want ErrNotFound", err)
	}

	renamed := b
	renamed.Name = "renamed"

	if err := database.UpdateSMSC(ctx, &renamed); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("UpdateSMSC renaming an SMSC = %v, want ErrNotFound", err)
	}
}

func TestSMSCNodesWithoutIdentity(t *testing.T) {
	ctx := context.Background()
	database := newSMSDatabase(t)

	for _, s := range []*db.SMSC{
		smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "", "192.0.2.1", 3868), node(nodeA2, "", "192.0.2.2", 3868)),
		smsc(smscB, "b", []string{"15550000002"}, node(nodeB1, "smsc.example.org", "192.0.2.1", 3869)),
	} {
		if err := database.CreateSMSC(ctx, s); err != nil {
			t.Fatalf("CreateSMSC(%s): %s", s.Name, err)
		}
	}

	got, err := database.GetSMSC(ctx, "a")
	if err != nil || got.Nodes[0].DiameterIdentity != "" || got.Nodes[1].DiameterIdentity != "" {
		t.Fatalf("GetSMSC = %+v, %v, want nodes without identity", got, err)
	}
}

func TestSMSCConflictsAreRejected(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name  string
		first *db.SMSC
		next  *db.SMSC
		want  error
	}{
		{
			"shared service centre",
			smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "smsc-192-0-2-1.example.org", "192.0.2.1", 3868)),
			smsc(smscB, "b", []string{"15550000000"}, node(nodeB1, "smsc-192-0-2-2.example.org", "192.0.2.2", 3868)),
			db.ErrSMSCConflict,
		},
		{
			"shared identity",
			smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "smsc.example.org", "192.0.2.1", 3868)),
			smsc(smscB, "b", []string{"15550000001"}, node(nodeB1, "SMSC.example.org", "192.0.2.2", 3868)),
			db.ErrSMSCConflict,
		},
		{
			"shared endpoint",
			smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "smsc-a.example.org", "192.0.2.1", 3868)),
			smsc(smscB, "b", []string{"15550000001"}, node(nodeB1, "smsc-b.example.org", "192.0.2.1", 3868)),
			db.ErrSMSCConflict,
		},
		{
			"shared address without identities",
			smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "", "192.0.2.1", 3868)),
			smsc(smscB, "b", []string{"15550000001"}, node(nodeB1, "", "192.0.2.1", 3869)),
			db.ErrSMSCConflict,
		},
		{
			"shared node ID",
			smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "smsc-a.example.org", "192.0.2.1", 3868)),
			smsc(smscB, "b", []string{"15550000001"}, node(nodeA1, "smsc-b.example.org", "192.0.2.2", 3868)),
			db.ErrSMSCConflict,
		},
		{
			"shared name",
			smsc(smscA, "main", []string{"15550000000"}, node(nodeA1, "smsc-a.example.org", "192.0.2.1", 3868)),
			smsc(smscB, "MAIN", []string{"15550000001"}, node(nodeB1, "smsc-b.example.org", "192.0.2.2", 3868)),
			db.ErrAlreadyExists,
		},
		{
			"two nodes of one SMSC on one endpoint",
			smsc(smscA, "a", []string{"15550000000"}, node(nodeA1, "smsc-a.example.org", "192.0.2.1", 3868)),
			smsc(smscB, "b", []string{"15550000001"}, node(nodeB1, "smsc-b1.example.org", "192.0.2.2", 3868), node(nodeB2, "smsc-b2.example.org", "192.0.2.2", 3868)),
			db.ErrSMSCConflict,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database := newSMSDatabase(t)

			if err := database.CreateSMSC(ctx, tc.first); err != nil {
				t.Fatalf("CreateSMSC: %s", err)
			}

			if err := database.CreateSMSC(ctx, tc.next); !errors.Is(err, tc.want) {
				t.Fatalf("CreateSMSC = %v, want %v", err, tc.want)
			}

			smscs, err := database.ListSMSCs(ctx)
			if err != nil || len(smscs) != 1 {
				t.Fatalf("ListSMSCs = %+v, %v, want only the first SMSC", smscs, err)
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
		{"default", db.DefaultSMSSettings(), true},
		{"number", db.SMSSettings{SMSNumber: "15550009999"}, true},
		{"no number", db.SMSSettings{}, true},
		{"plus sign", db.SMSSettings{SMSNumber: "+15550001111"}, false},
		{"leading zero", db.SMSSettings{SMSNumber: "0123"}, false},
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

func TestSMSCValidate(t *testing.T) {
	valid := *smsc(smscA, "main", []string{"15550000000"}, node(nodeA1, "smsc-192-0-2-1.example.org", "192.0.2.1", 3868))

	cases := []struct {
		name  string
		edit  func(s *db.SMSC)
		valid bool
	}{
		{"ipv4", func(*db.SMSC) {}, true},
		{"ipv6", func(s *db.SMSC) { s.Nodes[0].Address = "2001:db8::1" }, true},
		{"identity with an underscore", func(s *db.SMSC) { s.Nodes[0].DiameterIdentity = "smsc_1.example.org" }, true},
		{"no identity", func(s *db.SMSC) { s.Nodes[0].DiameterIdentity = "" }, true},
		{"eight nodes", func(s *db.SMSC) { s.Nodes = slices.Repeat(s.Nodes, db.MaxSMSCNodes) }, true},
		{"single label identity", func(s *db.SMSC) { s.Nodes[0].DiameterIdentity = "smsc" }, false},
		{"identity with a space", func(s *db.SMSC) { s.Nodes[0].DiameterIdentity = "smsc .example.org" }, false},
		{"non-canonical address", func(s *db.SMSC) { s.Nodes[0].Address = "2001:DB8::1" }, false},
		{"hostname", func(s *db.SMSC) { s.Nodes[0].Address = "smsc.example.org" }, false},
		{"unspecified", func(s *db.SMSC) { s.Nodes[0].Address = "0.0.0.0" }, false},
		{"zone", func(s *db.SMSC) { s.Nodes[0].Address = "fe80::1%eth0" }, false},
		{"port zero", func(s *db.SMSC) { s.Nodes[0].Port = 0 }, false},
		{"port too high", func(s *db.SMSC) { s.Nodes[0].Port = 65536 }, false},
		{"no node id", func(s *db.SMSC) { s.Nodes[0].ID = "" }, false},
		{"no nodes", func(s *db.SMSC) { s.Nodes = nil }, false},
		{"too many nodes", func(s *db.SMSC) { s.Nodes = slices.Repeat(s.Nodes, db.MaxSMSCNodes+1) }, false},
		{"no service centre", func(s *db.SMSC) { s.ServiceCentres = nil }, false},
		{"invalid service centre", func(s *db.SMSC) { s.ServiceCentres = []string{"+15550000000"} }, false},
		{"duplicate service centre", func(s *db.SMSC) { s.ServiceCentres = []string{"15550000000", "15550000000"} }, false},
		{"no id", func(s *db.SMSC) { s.ID = "" }, false},
		{"no name", func(s *db.SMSC) { s.Name = "" }, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := valid
			s.ServiceCentres = slices.Clone(valid.ServiceCentres)
			s.Nodes = slices.Clone(valid.Nodes)
			tc.edit(&s)

			if err := s.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate(%+v) = %v, want valid=%v", s, err, tc.valid)
			}
		})
	}
}

func smsc(id, name string, centres []string, nodes ...*db.SMSCNode) *db.SMSC {
	s := &db.SMSC{ID: id, Name: name, ServiceCentres: centres}
	for _, n := range nodes {
		s.Nodes = append(s.Nodes, *n)
	}

	return s
}

func node(id, identity, address string, port int) *db.SMSCNode {
	return &db.SMSCNode{ID: id, DiameterIdentity: identity, Address: address, Port: port}
}

func sameSMSC(x, y db.SMSC) bool {
	return x.ID == y.ID && x.Name == y.Name && slices.Equal(x.ServiceCentres, y.ServiceCentres) && slices.Equal(x.Nodes, y.Nodes)
}
