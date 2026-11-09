// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
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

func addrs(s ...string) []string {
	return s
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

	want := addrs("2001:db8::2", "10.0.0.6", "2001:db8::1", "10.0.0.5")

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true}, want); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	got, err := database.ListPCSCFAddresses(ctx)
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %s", err)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}

	reordered := addrs("10.0.0.5", "10.0.0.6")

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true}, reordered); err != nil {
		t.Fatalf("ReplacePCSCFAddresses reordering: %s", err)
	}

	got, err = database.ListPCSCFAddresses(ctx)
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %s", err)
	}

	if !slices.Equal(got, reordered) {
		t.Fatalf("addresses = %v, want %v", got, reordered)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true}, nil); err != nil {
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

	want := addrs("10.0.0.5")

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true}, want); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true}, addrs("10.0.0.6", "10.0.0.6")); err == nil {
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
		addresses []string
		valid     bool
	}{
		{"none", nil, true},
		{"five of each family", addrs("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5", "2001:db8::1", "2001:db8::2", "2001:db8::3", "2001:db8::4", "2001:db8::5"), true},
		{"six ipv4", addrs("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5", "10.0.0.6"), false},
		{"six ipv6", addrs("2001:db8::1", "2001:db8::2", "2001:db8::3", "2001:db8::4", "2001:db8::5", "2001:db8::6"), false},
		{"duplicate", addrs("10.0.0.1", "10.0.0.1"), false},
		{"unspecified", addrs("0.0.0.0"), false},
		{"loopback", addrs("::1"), false},
		{"multicast", addrs("224.0.0.1"), false},
		{"zone", addrs("fe80::1%eth0"), false},
		{"ipv4-mapped ipv6", addrs("::ffff:10.0.0.1"), false},
		{"ipv6 link-local", addrs("fe80::1"), false},
		{"ipv4 link-local", addrs("169.254.0.1"), false},
		{"limited broadcast", addrs("255.255.255.255"), false},
		{"empty", addrs(""), false},
		{"non-canonical ipv6", addrs("2001:DB8::1"), false},
		{"host name", addrs("pcscf.ims.mnc001.mcc001.3gppnetwork.org", "10.0.0.1"), true},
		{"single label host name", addrs("pcscf"), true},
		{"uppercase host name", addrs("PCSCF.example.org"), false},
		{"host name with an underscore", addrs("p_cscf.example.org"), false},
		{"duplicate host name", addrs("pcscf.example.org", "pcscf.example.org"), false},
		{"ten host names", addrs("a.example", "b.example", "c.example", "d.example", "e.example", "f.example", "g.example", "h.example", "i.example", "j.example"), true},
		{"eleven entries", addrs("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5", "2001:db8::1", "2001:db8::2", "2001:db8::3", "2001:db8::4", "2001:db8::5", "pcscf.example.org"), false},
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

		if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true}, addrs(addresses...)); err != nil {
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
		{[]string{"pcscf.example.org"}, db.IMSVoiceEnabled},
	}

	for _, tc := range cases {
		if got := status(tc.addresses...); got != tc.want {
			t.Errorf("P-CSCF %v on an IPv4-only ims data network: status = %q, want %q", tc.addresses, got, tc.want)
		}
	}

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: "mcx", IPv6Pool: "2001:db8:60::/48", DNS: "8.8.8.8", MTU: 1400, IMS: true}); err != nil {
		t.Fatalf("CreateDataNetwork mcx: %s", err)
	}

	if got := status("2001:db8::5"); got != db.IMSVoiceIPFamilyMismatch {
		t.Errorf("IPv6 P-CSCF with an IPv6 IMS data network other than ims: status = %q, want %q", got, db.IMSVoiceIPFamilyMismatch)
	}
}

func TestIMSVoiceStatusIgnoresNonIMSDataNetworks(t *testing.T) {
	ctx := context.Background()
	database := newPCSCFAddressesTestDatabase(t)

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: models.IMSDataNetworkName, IPv4Pool: "10.60.0.0/16", DNS: "8.8.8.8", MTU: 1400}); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true}, addrs("10.6.0.5")); err != nil {
		t.Fatalf("ReplacePCSCFAddresses: %s", err)
	}

	if got, err := database.IMSVoiceStatus(ctx); err != nil || got != db.IMSVoiceNoDataNetwork {
		t.Fatalf("status = %q, %v; want %q", got, err, db.IMSVoiceNoDataNetwork)
	}
}
