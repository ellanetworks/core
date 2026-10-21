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
	"github.com/ellanetworks/core/internal/models"
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
		{"ipv6 link-local", addrs(t, "fe80::1"), false},
		{"ipv4 link-local", addrs(t, "169.254.0.1"), false},
		{"limited broadcast", addrs(t, "255.255.255.255"), false},
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

func TestIMSVoiceStatus(t *testing.T) {
	ctx := context.Background()
	database := newPCSCFAddressesTestDatabase(t)

	status := func(addresses ...string) db.IMSVoiceStatus {
		t.Helper()

		if err := database.ReplacePCSCFAddresses(ctx, addrs(t, addresses...)); err != nil {
			t.Fatalf("ReplacePCSCFAddresses: %s", err)
		}

		s, err := database.IMSVoiceStatus(ctx)
		if err != nil {
			t.Fatalf("IMSVoiceStatus: %s", err)
		}

		return s
	}

	if got := status(); got != db.IMSVoiceNoPCSCF {
		t.Fatalf("no P-CSCF address: status = %q, want %q", got, db.IMSVoiceNoPCSCF)
	}

	if got := status("10.6.0.5"); got != db.IMSVoiceNoDataNetwork {
		t.Fatalf("no ims data network: status = %q, want %q", got, db.IMSVoiceNoDataNetwork)
	}

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: models.IMSDataNetworkName, IPv4Pool: "10.60.0.0/16", DNS: "8.8.8.8", MTU: 1400, IMS: true}); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	cases := []struct {
		addresses []string
		want      db.IMSVoiceStatus
	}{
		{nil, db.IMSVoiceNoPCSCF},
		{[]string{"2001:db8::5"}, db.IMSVoiceIPFamilyMismatch},
		{[]string{"2001:db8::5", "10.6.0.5"}, db.IMSVoiceEnabled},
	}

	for _, tc := range cases {
		if got := status(tc.addresses...); got != tc.want {
			t.Errorf("P-CSCF %v on an IPv4-only ims data network: status = %q, want %q", tc.addresses, got, tc.want)
		}
	}

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: "mcx", IPv6Pool: "2001:db8:60::/48", DNS: "8.8.8.8", MTU: 1400, IMS: true}); err != nil {
		t.Fatalf("CreateDataNetwork mcx: %s", err)
	}

	if got := status("2001:db8::5"); got != db.IMSVoiceEnabled {
		t.Errorf("IPv6 P-CSCF with an IPv6 IMS data network: status = %q, want %q", got, db.IMSVoiceEnabled)
	}
}

func TestIMSVoiceStatusIgnoresNonIMSDataNetworks(t *testing.T) {
	ctx := context.Background()
	database := newPCSCFAddressesTestDatabase(t)

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: models.IMSDataNetworkName, IPv4Pool: "10.60.0.0/16", DNS: "8.8.8.8", MTU: 1400}); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	if err := database.ReplacePCSCFAddresses(ctx, addrs(t, "10.6.0.5")); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	if got, err := database.IMSVoiceStatus(ctx); err != nil || got != db.IMSVoiceNoDataNetwork {
		t.Fatalf("status = %q, %v; want %q", got, err, db.IMSVoiceNoDataNetwork)
	}
}
