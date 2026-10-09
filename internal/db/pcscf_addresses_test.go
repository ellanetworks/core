// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"net/netip"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func newPCSCFAddressesTestDatabase(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

func addrs(t *testing.T, s ...string) []netip.Addr {
	t.Helper()

	out := make([]netip.Addr, 0, len(s))
	for _, a := range s {
		out = append(out, netip.MustParseAddr(a))
	}

	return out
}

func TestPCSCFAddressesDefaultToEmpty(t *testing.T) {
	database := newPCSCFAddressesTestDatabase(t)

	got, err := database.ListPCSCFAddresses(context.Background())
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %s", err)
	}

	if len(got) != 0 {
		t.Fatalf("addresses = %v, want none on a fresh database", got)
	}
}

func TestPCSCFAddressesKeepTheirOrder(t *testing.T) {
	ctx := context.Background()
	database := newPCSCFAddressesTestDatabase(t)

	want := addrs(t, "2001:db8::2", "10.0.0.6", "2001:db8::1", "10.0.0.5")

	if err := database.ReplacePCSCFAddresses(ctx, want); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	got, err := database.ListPCSCFAddresses(ctx)
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %s", err)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}

	reordered := addrs(t, "10.0.0.5", "10.0.0.6")

	if err := database.ReplacePCSCFAddresses(ctx, reordered); err != nil {
		t.Fatalf("ReplacePCSCFAddresses reordering: %s", err)
	}

	got, err = database.ListPCSCFAddresses(ctx)
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %s", err)
	}

	if !slices.Equal(got, reordered) {
		t.Fatalf("addresses = %v, want %v", got, reordered)
	}

	if err := database.ReplacePCSCFAddresses(ctx, nil); err != nil {
		t.Fatalf("ReplacePCSCFAddresses clearing: %s", err)
	}

	got, err = database.ListPCSCFAddresses(ctx)
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %s", err)
	}

	if len(got) != 0 {
		t.Fatalf("addresses = %v, want none after clearing", got)
	}
}

func TestPCSCFAddressesRejectedListLeavesStoredOne(t *testing.T) {
	ctx := context.Background()
	database := newPCSCFAddressesTestDatabase(t)

	want := addrs(t, "10.0.0.5")

	if err := database.ReplacePCSCFAddresses(ctx, want); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	if err := database.ReplacePCSCFAddresses(ctx, addrs(t, "10.0.0.6", "10.0.0.6")); err == nil {
		t.Fatal("duplicate addresses accepted")
	}

	got, err := database.ListPCSCFAddresses(ctx)
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %s", err)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}
}

func TestValidatePCSCFAddresses(t *testing.T) {
	cases := []struct {
		name      string
		addresses []netip.Addr
		valid     bool
	}{
		{"none", nil, true},
		{"three of each family", addrs(t, "10.0.0.1", "10.0.0.2", "10.0.0.3", "2001:db8::1", "2001:db8::2", "2001:db8::3"), true},
		{"four ipv4", addrs(t, "10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4"), false},
		{"four ipv6", addrs(t, "2001:db8::1", "2001:db8::2", "2001:db8::3", "2001:db8::4"), false},
		{"duplicate", addrs(t, "10.0.0.1", "10.0.0.1"), false},
		{"unspecified", addrs(t, "0.0.0.0"), false},
		{"loopback", addrs(t, "::1"), false},
		{"multicast", addrs(t, "224.0.0.1"), false},
		{"zone", addrs(t, "fe80::1%eth0"), false},
		{"ipv4-mapped ipv6", addrs(t, "::ffff:10.0.0.1"), false},
		{"invalid", []netip.Addr{{}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := db.ValidatePCSCFAddresses(tc.addresses)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidatePCSCFAddresses(%v) = %v, want valid=%v", tc.addresses, err, tc.valid)
			}
		})
	}
}
