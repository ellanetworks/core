// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestIMSHomeDomainDefaultsToTheDerivedDomain(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	if err := database.UpdateOperatorID(ctx, "001", "01"); err != nil {
		t.Fatalf("set operator ID: %v", err)
	}

	domains, err := database.GetIMSHomeDomains(ctx)
	if err != nil {
		t.Fatalf("home domains: %v", err)
	}

	want := db.IMSHomeDomains{Primary: "ims.mnc001.mcc001.3gppnetwork.org", Derived: "ims.mnc001.mcc001.3gppnetwork.org"}
	if domains != want || !slices.Equal(domains.Accepted(), []string{want.Primary}) {
		t.Fatalf("domains = %+v, accepted %v", domains, domains.Accepted())
	}
}

func TestIMSHomeDomainValidation(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	for _, domain := range []string{"localhost", "192.0.2.1", "2001:db8::1", "ims_example.com", "-ims.example.com", "ims..example.com"} {
		if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: domain}, nil); err == nil {
			t.Fatalf("%q was accepted", domain)
		}
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: " IMS.Example.COM "}, nil); err != nil {
		t.Fatalf("set home domain: %v", err)
	}

	settings, err := database.GetIMSSettings(ctx)
	if err != nil || settings.HomeDomain != "ims.example.com" {
		t.Fatalf("settings = %+v, err %v", settings, err)
	}
}

func TestIMSApplicationServersFollowTheHomeDomain(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	if err := database.UpdateOperatorID(ctx, "001", "01"); err != nil {
		t.Fatalf("set operator ID: %v", err)
	}

	as := &db.IMSApplicationServer{
		Name:                    "mcptt",
		ServerName:              "sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org",
		PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.mnc001.mcc001.3gppnetwork.org"},
	}

	if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
		t.Fatalf("create: %v", err)
	}

	requireAS := func(t *testing.T, serverName, psi string) {
		t.Helper()

		got, err := database.GetIMSApplicationServer(ctx, "mcptt")
		if err != nil {
			t.Fatalf("get: %v", err)
		}

		if got.ServerName != serverName || !slices.Equal(got.PublicServiceIdentities, []string{psi}) {
			t.Fatalf("application server = %+v, want %s %s", got, serverName, psi)
		}
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: "ims.example.com"}, nil); err != nil {
		t.Fatalf("set home domain: %v", err)
	}

	requireAS(t, "sip:mcptt.ims.example.com", "sip:!.*!@mcptt.ims.example.com")

	if err := database.UpdateOperatorID(ctx, "999", "010"); err != nil {
		t.Fatalf("update operator ID: %v", err)
	}

	requireAS(t, "sip:mcptt.ims.example.com", "sip:!.*!@mcptt.ims.example.com")

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{}, nil); err != nil {
		t.Fatalf("reset home domain: %v", err)
	}

	requireAS(t, "sip:mcptt.ims.mnc010.mcc999.3gppnetwork.org", "sip:!.*!@mcptt.ims.mnc010.mcc999.3gppnetwork.org")
}

func TestPSIConflictsCoverBothHomeDomains(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	if err := database.UpdateOperatorID(ctx, "001", "01"); err != nil {
		t.Fatalf("set operator ID: %v", err)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: "ims.example.com"}, nil); err != nil {
		t.Fatalf("set home domain: %v", err)
	}

	for _, psi := range []string{"sip:15551230001@ims.example.com", "sip:15551230001@ims.mnc001.mcc001.3gppnetwork.org"} {
		err := database.CreateIMSApplicationServer(ctx, &db.IMSApplicationServer{Name: "as", ServerName: "sip:as.example.org", PublicServiceIdentities: []string{psi}})
		if !errors.Is(err, db.ErrPSIConflict) {
			t.Fatalf("%s: err = %v, want a PSI conflict", psi, err)
		}
	}
}

func TestIMSPeers(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	if err := database.CreateIMSPeer(ctx, &db.IMSPeer{ID: "1", Name: "scscf", OriginHost: "scscf.ims.example.org", Addresses: []string{"192.0.2.0/24"}}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := database.CreateIMSPeer(ctx, &db.IMSPeer{ID: "2", Name: "realm", Realm: "ims.example.org", Addresses: []string{"0.0.0.0/0"}}); err != nil {
		t.Fatalf("create realm peer: %v", err)
	}

	for name, peer := range map[string]db.IMSPeer{
		"same name":            {ID: "3", Name: "SCSCF", Realm: "other.example.org", Addresses: []string{"192.0.2.1"}},
		"same Origin-Host":     {ID: "3", Name: "other", OriginHost: "scscf.ims.example.org", Addresses: []string{"192.0.2.0/24"}},
		"same realm":           {ID: "3", Name: "other", Realm: "ims.example.org", Addresses: []string{"0.0.0.0/0"}},
		"same rule normalized": {ID: "3", Name: "other", Realm: "ims.example.org", Addresses: []string{"0.0.0.0/0", "0.1.0.0/0"}},
	} {
		err := database.CreateIMSPeer(ctx, &peer)
		if !errors.Is(err, db.ErrAlreadyExists) && !errors.Is(err, db.ErrIMSPeerConflict) {
			t.Fatalf("%s: err = %v, want a conflict", name, err)
		}
	}

	if err := database.CreateIMSPeer(ctx, &db.IMSPeer{ID: "5", Name: "scscf-site-b", OriginHost: "scscf.ims.example.org", Addresses: []string{"10.0.0.0/8"}}); err != nil {
		t.Fatalf("the same Origin-Host from other addresses is another rule: %v", err)
	}

	if err := database.CreateIMSPeer(ctx, &db.IMSPeer{ID: "4", Name: "none", Addresses: []string{"192.0.2.1"}}); err == nil {
		t.Fatal("a peer without Origin-Host or realm was accepted")
	}

	if err := database.CreateIMSPeer(ctx, &db.IMSPeer{ID: "4", Name: "addresses-only", Addresses: []string{"10.0.0.1"}}); err == nil {
		t.Fatal("a peer with only addresses was accepted")
	}

	if err := database.CreateIMSPeer(ctx, &db.IMSPeer{ID: "4", Name: "bad", Realm: "bad.example.org", Addresses: []string{"scscf.example.org"}}); err == nil {
		t.Fatal("a peer with a host name as address was accepted")
	}

	if err := database.CreateIMSPeer(ctx, &db.IMSPeer{ID: "4", Name: "no-address", Realm: "bad.example.org"}); err == nil {
		t.Fatal("a peer without a source address was accepted")
	}

	if err := database.UpdateIMSPeer(ctx, &db.IMSPeer{ID: "1", Name: "scscf", Realm: "scscf.example.org", Addresses: []string{"2001:db8::1", "192.0.2.77/24", "192.0.2.0/24", "::ffff:198.51.100.7"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	peer, err := database.GetIMSPeer(ctx, "SCSCF")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if want := []string{"192.0.2.0/24", "198.51.100.7/32", "2001:db8::1/128"}; peer.OriginHost != "" || peer.Realm != "scscf.example.org" || !slices.Equal(peer.Addresses, want) {
		t.Fatalf("peer = %+v, want addresses %v", peer, want)
	}

	for _, tc := range []struct {
		host, realm, remote string
		want                bool
	}{
		{"any.example.org", "SCSCF.example.org", "192.0.2.9", true},
		{"any.example.org", "scscf.example.org", "2001:db8::1", true},
		{"any.example.org", "scscf.example.org", "::ffff:198.51.100.7", true},
		{"any.example.org", "scscf.example.org", "198.51.100.8", false},
		{"any.example.org", "other.example.org", "192.0.2.9", false},
	} {
		if got := peer.Rule().Matches(tc.host, tc.realm, netip.MustParseAddr(tc.remote)); got != tc.want {
			t.Fatalf("Matches(%s, %s, %s) = %v, want %v", tc.host, tc.realm, tc.remote, got, tc.want)
		}
	}

	realm, err := database.GetIMSPeer(ctx, "realm")
	if err != nil || !slices.Equal(realm.Addresses, []string{"0.0.0.0/0"}) {
		t.Fatalf("realm peer = %+v, err %v, want 0.0.0.0/0", realm, err)
	}

	if err := database.DeleteIMSPeer(ctx, "1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := database.GetIMSPeer(ctx, "scscf"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("get deleted: err = %v", err)
	}
}

func TestIMSPeerMatchesEveryFieldThatIsSet(t *testing.T) {
	remote := netip.MustParseAddr("10.1.2.3")

	for name, tc := range map[string]struct {
		peer db.IMSPeer
		want bool
	}{
		"host":                     {db.IMSPeer{OriginHost: "scscf.example.org", Addresses: []string{"0.0.0.0/0"}}, true},
		"host, other realm":        {db.IMSPeer{OriginHost: "scscf.example.org", Realm: "other.example.org", Addresses: []string{"0.0.0.0/0"}}, false},
		"host and realm":           {db.IMSPeer{OriginHost: "scscf.example.org", Realm: "example.org", Addresses: []string{"0.0.0.0/0"}}, true},
		"host, address inside":     {db.IMSPeer{OriginHost: "scscf.example.org", Addresses: []string{"10.0.0.0/8"}}, true},
		"host, address outside":    {db.IMSPeer{OriginHost: "scscf.example.org", Addresses: []string{"10.2.0.0/16"}}, false},
		"realm, one of addresses":  {db.IMSPeer{Realm: "example.org", Addresses: []string{"10.2.0.0/16", "10.1.2.3/32"}}, true},
		"other host":               {db.IMSPeer{OriginHost: "pcscf.example.org", Addresses: []string{"0.0.0.0/0"}}, false},
		"addresses only":           {db.IMSPeer{Addresses: []string{"10.0.0.0/8"}}, false},
		"IPv6 prefix, IPv4 remote": {db.IMSPeer{Realm: "example.org", Addresses: []string{"::/0"}}, false},
	} {
		if got := tc.peer.Rule().Matches("SCSCF.example.org", "example.org", remote); got != tc.want {
			t.Fatalf("%s: Matches = %v, want %v", name, got, tc.want)
		}
	}
}

func TestNormalizeIMSPeerAddresses(t *testing.T) {
	got, err := db.NormalizeIMSPeerAddresses([]string{" 10.0.0.5/8 ", "10.0.0.0/8", "fe80::1", "::ffff:10.0.0.0/104"})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	if want := []string{"10.0.0.0/8", "fe80::1/128"}; !slices.Equal(got, want) {
		t.Fatalf("normalized = %v, want %v", got, want)
	}

	for _, bad := range []string{"", "example.org", "10.0.0.0/33", "fe80::1%eth0", "::ffff:10.0.0.0/64"} {
		if _, err := db.NormalizeIMSPeerAddresses([]string{bad}); err == nil {
			t.Fatalf("%q was accepted", bad)
		}
	}

	if _, err := db.NormalizeIMSPeerAddresses(make([]string, db.MaxIMSPeerAddresses+1)); err == nil {
		t.Fatal("too many addresses were accepted")
	}
}

func TestIMSSettingsAdvertiseVoiceByDefault(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	settings, err := database.GetIMSSettings(ctx)
	if err != nil || !settings.AdvertiseVoice || settings.HomeDomain != "" || settings.RestrictPeers {
		t.Fatalf("default IMS settings = %+v, %v; want IMS voice advertised and any peer admitted", settings, err)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: "ims.example.com", AdvertiseVoice: false}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	settings, err = database.GetIMSSettings(ctx)
	if err != nil || settings.AdvertiseVoice || settings.HomeDomain != "ims.example.com" {
		t.Fatalf("IMS settings = %+v, %v", settings, err)
	}
}

func TestUpdateIMSConfigurationIsAtomic(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: "ims.example.com", AdvertiseVoice: true}, []string{"10.70.0.5", "pcscf.example.org"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	addresses, err := database.ListPCSCFAddresses(ctx)
	if err != nil || !slices.Equal(addresses, []string{"10.70.0.5", "pcscf.example.org"}) {
		t.Fatalf("P-CSCF addresses = %v, %v", addresses, err)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: "not a domain"}, []string{"10.70.0.6"}); err == nil {
		t.Fatal("an invalid home domain was accepted")
	}

	addresses, err = database.ListPCSCFAddresses(ctx)
	if err != nil || !slices.Equal(addresses, []string{"10.70.0.5", "pcscf.example.org"}) {
		t.Fatalf("P-CSCF addresses after a rejected update = %v, %v; want them unchanged", addresses, err)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: false}, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}

	addresses, _ = database.ListPCSCFAddresses(ctx)
	settings, _ := database.GetIMSSettings(ctx)

	if len(addresses) != 0 || settings.HomeDomain != "" || settings.AdvertiseVoice {
		t.Fatalf("after clearing: addresses %v, settings %+v", addresses, settings)
	}
}

func TestIMSSettingsRestrictPeers(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{AdvertiseVoice: true, RestrictPeers: true}, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	settings, err := database.GetIMSSettings(ctx)
	if err != nil || !settings.RestrictPeers || !settings.AdvertiseVoice {
		t.Fatalf("IMS settings = %+v, %v; want peers restricted", settings, err)
	}
}

func TestNormalizeIMSHomeDomainReservesThe3GPPNetworkDomains(t *testing.T) {
	for domain, want := range map[string]string{
		"ims.example.com":                        "ims.example.com",
		"IMS.mnc001.mcc001.3gppnetwork.org":      "",
		"core.ims.mnc001.mcc001.3gppnetwork.org": "core.ims.mnc001.mcc001.3gppnetwork.org",
	} {
		got, err := db.NormalizeIMSHomeDomain(domain, "001", "01")
		if err != nil || got != want {
			t.Fatalf("NormalizeIMSHomeDomain(%q) = %q, %v; want %q", domain, got, err, want)
		}
	}

	for _, domain := range []string{
		"epc.mnc001.mcc001.3gppnetwork.org",
		"ims.mnc093.mcc208.3gppnetwork.org",
		"3gppnetwork.org",
		"pub.3gppnetwork.org",
		"ims.mnc01.mcc001.3gppnetwork.org",
	} {
		if got, err := db.NormalizeIMSHomeDomain(domain, "001", "01"); err == nil {
			t.Fatalf("NormalizeIMSHomeDomain(%q) = %q, want an error", domain, got)
		}
	}
}

func TestIMSHomeDomainEqualToTheDerivedDomainFollowsThePLMN(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	if err := database.UpdateOperatorID(ctx, "001", "01"); err != nil {
		t.Fatalf("set operator ID: %v", err)
	}

	if err := database.UpdateIMSConfiguration(ctx, &db.IMSSettings{HomeDomain: "ims.mnc001.mcc001.3gppnetwork.org"}, nil); err != nil {
		t.Fatalf("set home domain: %v", err)
	}

	if err := database.UpdateOperatorID(ctx, "999", "010"); err != nil {
		t.Fatalf("update operator ID: %v", err)
	}

	domains, err := database.GetIMSHomeDomains(ctx)
	if err != nil || domains.Primary != "ims.mnc010.mcc999.3gppnetwork.org" {
		t.Fatalf("home domains = %+v, %v; want the new derived domain", domains, err)
	}
}
