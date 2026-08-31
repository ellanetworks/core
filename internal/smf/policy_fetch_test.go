// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/pkg/runtime"
)

const (
	policyFixtureDNN  = "test-dnn"
	policyFixtureIMSI = "310410000000001"
)

func setupPolicyFixture(t *testing.T, ambrUplink, ambrDownlink string) (*db.Database, *db.Policy) {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabase(ctx, filepath.Join(t.TempDir(), "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("couldn't create test database: %s", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %v", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("couldn't close database: %s", err)
		}
	})

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: policyFixtureDNN, IPv4Pool: "10.1.0.0/24"}); err != nil {
		t.Fatalf("couldn't create test data network: %s", err)
	}

	dataNetwork, err := database.GetDataNetwork(ctx, policyFixtureDNN)
	if err != nil {
		t.Fatalf("couldn't get test data network: %s", err)
	}

	if err := database.CreateProfile(ctx, &db.Profile{
		Name: "test-profile", UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps",
	}); err != nil {
		t.Fatalf("couldn't create test profile: %s", err)
	}

	profile, err := database.GetProfile(ctx, "test-profile")
	if err != nil {
		t.Fatalf("couldn't get test profile: %s", err)
	}

	if err := database.CreateNetworkSlice(ctx, &db.NetworkSlice{Name: "test-slice", Sst: 1}); err != nil {
		t.Fatalf("couldn't create test slice: %s", err)
	}

	slice, err := database.GetNetworkSlice(ctx, "test-slice")
	if err != nil {
		t.Fatalf("couldn't get test slice: %s", err)
	}

	if err := database.CreatePolicy(ctx, &db.Policy{
		Name:                "test-policy",
		SessionAmbrUplink:   ambrUplink,
		SessionAmbrDownlink: ambrDownlink,
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profile.ID,
		SliceID:             slice.ID,
	}); err != nil {
		t.Fatalf("couldn't create test policy: %s", err)
	}

	policy, err := database.GetPolicy(ctx, "test-policy")
	if err != nil {
		t.Fatalf("couldn't get created policy: %s", err)
	}

	if err := database.CreateSubscriber(ctx, &db.Subscriber{
		Imsi:           policyFixtureIMSI,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      policy.ProfileID,
	}); err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	return database, policy
}

func fetchSessionPolicy(t *testing.T, database *db.Database) *smf.Policy {
	t.Helper()

	adapter := runtime.NewPCFDBAdapter(database)

	policy, err := adapter.GetSessionPolicy(context.Background(), policyFixtureIMSI,
		&models.Snssai{Sst: db.InitialSliceSst, Sd: ""}, policyFixtureDNN)
	if err != nil {
		t.Fatalf("GetSessionPolicy failed: %v", err)
	}

	if policy == nil {
		t.Fatal("expected non-nil policy")
	}

	return policy
}

func TestGetSessionPolicy_FetchesNetworkRules(t *testing.T) {
	database, created := setupPolicyFixture(t, "100 Mbps", "200 Mbps")
	ctx := context.Background()

	prefix1 := "192.168.0.0/24"
	prefix2 := "10.0.0.0/8"

	for _, rule := range []*db.NetworkRule{
		{
			PolicyID: created.ID, Description: "rule-1", Direction: "uplink",
			RemotePrefix: &prefix1, Protocol: 6, PortLow: 80, PortHigh: 443,
			Action: "allow", Precedence: 1,
		},
		{
			PolicyID: created.ID, Description: "rule-2", Direction: "downlink",
			RemotePrefix: &prefix2, Protocol: 17, PortLow: 5060, PortHigh: 5060,
			Action: "deny", Precedence: 2,
		},
	} {
		id, err := database.CreateNetworkRule(ctx, rule)
		if err != nil {
			t.Fatalf("couldn't create %s: %s", rule.Description, err)
		}

		if id == "" {
			t.Fatalf("%s: expected non-empty rule ID", rule.Description)
		}
	}

	policy := fetchSessionPolicy(t, database)

	if !policy.Ambr.Uplink.Equal(models.MustParseBitRate("100 Mbps")) {
		t.Fatalf("expected uplink 100 Mbps, got %s", policy.Ambr.Uplink)
	}

	want := map[string]smf.ResolvedNetworkRule{
		"rule-1": {
			Description: "rule-1", PolicyID: created.ID, Direction: models.DirectionUplink,
			RemotePrefix: &prefix1, Protocol: 6, PortLow: 80, PortHigh: 443,
			Action: "allow", Precedence: 1,
		},
		"rule-2": {
			Description: "rule-2", PolicyID: created.ID, Direction: models.DirectionDownlink,
			RemotePrefix: &prefix2, Protocol: 17, PortLow: 5060, PortHigh: 5060,
			Action: "deny", Precedence: 2,
		},
	}

	if len(policy.NetworkRules) != len(want) {
		t.Fatalf("expected %d network rules, got %d", len(want), len(policy.NetworkRules))
	}

	for _, got := range policy.NetworkRules {
		expected, ok := want[got.Description]
		if !ok {
			t.Errorf("unexpected network rule %q", got.Description)
			continue
		}

		if !reflect.DeepEqual(*got, expected) {
			t.Errorf("%s = %+v, want %+v", got.Description, *got, expected)
		}

		delete(want, got.Description)
	}

	for description := range want {
		t.Errorf("%s not found in network rules", description)
	}
}

func TestGetSessionPolicy_NoNetworkRules(t *testing.T) {
	database, _ := setupPolicyFixture(t, "50 Mbps", "100 Mbps")

	if got := fetchSessionPolicy(t, database).NetworkRules; len(got) != 0 {
		t.Fatalf("expected 0 network rules, got %d", len(got))
	}
}
