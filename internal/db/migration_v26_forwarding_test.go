// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

type forwardingFixture struct {
	slice, internet, ims, unused string
}

func setupForwardingFixture(t *testing.T, d *Database) forwardingFixture {
	t.Helper()

	ctx := context.Background()

	if err := d.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	slice, err := d.GetNetworkSlice(ctx, InitialSliceName)
	if err != nil {
		t.Fatalf("get default slice: %v", err)
	}

	profile, err := d.GetProfile(ctx, InitialProfileName)
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}

	internet, err := d.GetDataNetwork(ctx, InitialDataNetworkName)
	if err != nil {
		t.Fatalf("get default data network: %v", err)
	}

	ims := &DataNetwork{Name: models.IMSDataNetworkName, IPv4Pool: "10.46.0.0/22", DNS: "8.8.8.8", MTU: 1400, IMS: true}
	unused := &DataNetwork{Name: "unused", IPv4Pool: "10.47.0.0/22", DNS: "8.8.8.8", MTU: 1400}

	for _, dn := range []*DataNetwork{ims, unused} {
		if err := d.CreateDataNetwork(ctx, dn); err != nil {
			t.Fatalf("create %s: %v", dn.Name, err)
		}
	}

	if err := d.CreatePolicy(ctx, &Policy{
		Name: "ims", ProfileID: profile.ID, SliceID: slice.ID, DataNetworkID: ims.ID,
		Var5qi: models.IMSSignalling5QI, Arp: 1, SessionAmbrUplink: "1 Mbps", SessionAmbrDownlink: "1 Mbps",
		PreemptionCapability: models.PreemptionCapabilityNotPreempt, PreemptionVulnerability: models.PreemptionVulnerabilityNotPreemptable,
	}); err != nil {
		t.Fatalf("create ims policy: %v", err)
	}

	return forwardingFixture{slice: slice.ID, internet: internet.ID, ims: ims.ID, unused: unused.ID}
}

func TestMigrationV26MovesNATAndLocalSwitchToSliceDataNetworks(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		nat, localSwitch     bool
		leaderLocalSwitch    *bool
		leaderNAT            *bool
		wantNATAtV26         bool
		wantLocalSwitchAtV26 bool
	}{
		{name: "standalone defaults", nat: true, localSwitch: false, wantNATAtV26: true, wantLocalSwitchAtV26: false},
		{name: "standalone with NAT off and local switching on", nat: false, localSwitch: true, wantNATAtV26: false, wantLocalSwitchAtV26: true},
		{name: "cluster uses the leader's settings", nat: true, localSwitch: true, leaderLocalSwitch: new(false), leaderNAT: new(false), wantNATAtV26: false, wantLocalSwitchAtV26: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			d := newDatabaseAtVersion(t, 25)

			setLegacyToggle(t, d, NATSettingsTableName, tc.nat)
			setLegacyToggle(t, d, LocalSwitchSettingsTableName, tc.localSwitch)

			f := setupForwardingFixture(t, d)

			assertForwarding(t, d, "at v25", f.slice, f.internet, tc.nat, tc.localSwitch)
			assertForwarding(t, d, "at v25", f.slice, f.ims, false, true)
			assertNoForwarding(t, d, "at v25", f.slice, f.unused)

			if err := d.CreateSliceDataNetwork(ctx, &SliceDataNetwork{SliceID: f.slice, DataNetworkID: f.unused}); err == nil {
				t.Fatal("CreateSliceDataNetwork at v25 succeeded, want a pending-migration error")
			}

			if err := runMigrations(withMigrationToggles(ctx, tc.leaderLocalSwitch, tc.leaderNAT), d.PlainDB(), 26); err != nil {
				t.Fatalf("runMigrations(26): %v", err)
			}

			if err := d.refreshAppliedSchema(ctx); err != nil {
				t.Fatalf("refresh applied schema: %v", err)
			}

			assertForwarding(t, d, "at v26", f.slice, f.internet, tc.wantNATAtV26, tc.wantLocalSwitchAtV26)
			assertForwarding(t, d, "at v26", f.slice, f.ims, false, true)
			assertNoForwarding(t, d, "at v26", f.slice, f.unused)

			if err := d.UpdateSliceDataNetwork(ctx, &SliceDataNetwork{SliceID: f.slice, DataNetworkID: f.internet, NAT: !tc.wantNATAtV26, LocalSwitch: !tc.wantLocalSwitchAtV26}); err != nil {
				t.Fatalf("UpdateSliceDataNetwork at v26: %v", err)
			}

			assertForwarding(t, d, "after edits", f.slice, f.internet, !tc.wantNATAtV26, !tc.wantLocalSwitchAtV26)

			if err := d.DeleteSliceDataNetwork(ctx, f.slice, f.internet); !errors.Is(err, ErrInUse) {
				t.Fatalf("delete a slice data network with policies = %v, want ErrInUse", err)
			}

			if err := d.CreateSliceDataNetwork(ctx, &SliceDataNetwork{SliceID: f.slice, DataNetworkID: f.unused, NAT: true}); err != nil {
				t.Fatalf("serve an unused data network: %v", err)
			}

			assertForwarding(t, d, "new pair", f.slice, f.unused, true, false)

			if err := d.DeleteDataNetwork(ctx, "unused"); err != nil {
				t.Fatalf("delete a data network served by a slice: %v", err)
			}

			assertNoForwarding(t, d, "after deleting the data network", f.slice, f.unused)
		})
	}
}

func TestFreshDatabaseServesTheDefaultDataNetwork(t *testing.T) {
	d := newDatabaseAtVersion(t, 26)
	f := setupForwardingFixture(t, d)

	assertForwarding(t, d, "fresh", f.slice, f.internet, NATDefaultEnabled, false)
}

func setLegacyToggle(t *testing.T, d *Database, table string, enabled bool) {
	t.Helper()

	stmt := fmt.Sprintf("INSERT INTO %s (singleton, enabled) VALUES (TRUE, ?) ON CONFLICT(singleton) DO UPDATE SET enabled = excluded.enabled", table)
	if _, err := d.PlainDB().ExecContext(context.Background(), stmt, enabled); err != nil {
		t.Fatalf("set %s: %v", table, err)
	}
}

func assertForwarding(t *testing.T, d *Database, label, sliceID, dataNetworkID string, wantNAT, wantLocalSwitch bool) {
	t.Helper()

	f, err := d.GetSliceDataNetworkForwarding(context.Background(), sliceID, dataNetworkID)
	if err != nil {
		t.Fatalf("%s: get forwarding of %s/%s: %v", label, sliceID, dataNetworkID, err)
	}

	if f.NAT != wantNAT || f.LocalSwitch != wantLocalSwitch {
		t.Fatalf("%s: %s/%s NAT %v, local switch %v; want %v, %v", label, sliceID, dataNetworkID, f.NAT, f.LocalSwitch, wantNAT, wantLocalSwitch)
	}

	if f.NetworkInstance != models.DefaultNetworkInstanceName {
		t.Fatalf("%s: network instance %q", label, f.NetworkInstance)
	}
}

func assertNoForwarding(t *testing.T, d *Database, label, sliceID, dataNetworkID string) {
	t.Helper()

	if _, err := d.GetSliceDataNetworkForwarding(context.Background(), sliceID, dataNetworkID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("%s: %s/%s served (%v), want not served", label, sliceID, dataNetworkID, err)
	}
}

func TestWithheldLeaseOwnersFollowTheSliceDataNetwork(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 26)
	f := setupForwardingFixture(t, d)

	profile, err := d.GetProfile(ctx, InitialProfileName)
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}

	if err := d.CreateSliceDataNetwork(ctx, &SliceDataNetwork{SliceID: f.slice, DataNetworkID: f.ims, LocalSwitch: true}); err != nil {
		t.Fatalf("serve ims: %v", err)
	}

	const imsi = "001010000000042"

	if err := d.CreateSubscriber(ctx, &Subscriber{
		Imsi: imsi, SequenceNumber: "000000000022", PermanentKey: "6f30087629feb0b089783c81d0ae09b5",
		Opc: "21a7e1897dfb481d62439142cdf1b6ee", ProfileID: profile.ID,
	}); err != nil {
		t.Fatalf("create subscriber: %v", err)
	}

	for i, dn := range []string{f.internet, f.ims} {
		if _, err := d.AllocateIPLease(ctx, dn, "ipv4", imsi, i+1, "1"); err != nil {
			t.Fatalf("allocate a lease on %s: %v", dn, err)
		}
	}

	assertWithheld := func(label string, want map[string]LeaseWithholding) {
		t.Helper()

		got, err := d.WithheldLeaseOwners(ctx, "1")
		if err != nil {
			t.Fatalf("%s: WithheldLeaseOwners: %v", label, err)
		}

		if !maps.Equal(got, want) {
			t.Fatalf("%s: withheld lease owners %v, want %v", label, got, want)
		}
	}

	assertWithheld("NAT on internet", map[string]LeaseWithholding{imsi + "/" + f.internet: {IPv4: true}, imsi + "/" + f.ims: {}})

	if err := d.UpdateSliceDataNetwork(ctx, &SliceDataNetwork{SliceID: f.slice, DataNetworkID: f.internet}); err != nil {
		t.Fatalf("turn NAT off: %v", err)
	}

	assertWithheld("NAT off", map[string]LeaseWithholding{imsi + "/" + f.internet: {}, imsi + "/" + f.ims: {}})

	if err := d.UpdateSliceDataNetwork(ctx, &SliceDataNetwork{SliceID: f.slice, DataNetworkID: f.ims, LocalSwitch: true, NetworkInstance: "ims"}); err != nil {
		t.Fatalf("move ims to a network instance: %v", err)
	}

	assertWithheld("ims on a network instance", map[string]LeaseWithholding{imsi + "/" + f.internet: {}, imsi + "/" + f.ims: {IPv4: true, IPv6: true}})

	if got, err := d.WithheldLeaseOwners(ctx, "2"); err != nil || len(got) != 0 {
		t.Fatalf("leases of another node = %v, %v; want none", got, err)
	}
}
