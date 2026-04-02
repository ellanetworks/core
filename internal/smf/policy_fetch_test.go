// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/pkg/runtime"
)

func TestGetSessionPolicy_FetchesNetworkRules(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test database: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("couldn't close database: %s", err)
		}
	}()

	ctx := context.Background()

	testDN := &db.DataNetwork{Name: "test-dnn", IPPool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(ctx, testDN); err != nil {
		t.Fatalf("couldn't create test data network: %s", err)
	}

	testDataNetwork, err := database.GetDataNetwork(ctx, "test-dnn")
	if err != nil {
		t.Fatalf("couldn't get test data network: %s", err)
	}

	testProfile := &db.Profile{Name: "test-profile", UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps"}
	if err := database.CreateProfile(ctx, testProfile); err != nil {
		t.Fatalf("couldn't create test profile: %s", err)
	}

	createdProfile, err := database.GetProfile(ctx, "test-profile")
	if err != nil {
		t.Fatalf("couldn't get test profile: %s", err)
	}

	policy := &db.Policy{
		Name:                "test-policy",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       testDataNetwork.ID,
		ProfileID:           createdProfile.ID,
		SliceID:             1,
	}

	err = database.CreatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("couldn't create test policy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(ctx, "test-policy")
	if err != nil {
		t.Fatalf("couldn't get created policy: %s", err)
	}

	prefix1 := "192.168.0.0/24"
	rule1 := &db.NetworkRule{
		PolicyID:     int64(createdPolicy.ID),
		Description:  "rule-1",
		Direction:    "uplink",
		RemotePrefix: &prefix1,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "allow",
		Precedence:   1,
	}

	id1, err := database.CreateNetworkRule(ctx, rule1)
	if err != nil {
		t.Fatalf("couldn't create rule 1: %s", err)
	}

	if id1 == 0 {
		t.Fatalf("expected non-zero rule ID")
	}

	prefix2 := "10.0.0.0/8"
	rule2 := &db.NetworkRule{
		PolicyID:     int64(createdPolicy.ID),
		Description:  "rule-2",
		Direction:    "downlink",
		RemotePrefix: &prefix2,
		Protocol:     17,
		PortLow:      5060,
		PortHigh:     5060,
		Action:       "deny",
		Precedence:   2,
	}

	id2, err := database.CreateNetworkRule(ctx, rule2)
	if err != nil {
		t.Fatalf("couldn't create rule 2: %s", err)
	}

	if id2 == 0 {
		t.Fatalf("expected non-zero rule ID")
	}

	subscriber := &db.Subscriber{
		Imsi:           "310410000000001",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      createdPolicy.ProfileID,
	}

	err = database.CreateSubscriber(ctx, subscriber)
	if err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	adapter := runtime.NewSMFDBAdapter(database)

	snssai := &models.Snssai{Sst: db.InitialSliceSst, Sd: db.InitialSliceSd}

	retrievedPolicy, err := adapter.GetSessionPolicy(ctx, "310410000000001", snssai, "test-dnn")
	if err != nil {
		t.Fatalf("GetSessionPolicy failed: %v", err)
	}

	if retrievedPolicy == nil {
		t.Fatalf("expected non-nil policy")
	}

	if retrievedPolicy.Ambr.Uplink != "100 Mbps" {
		t.Fatalf("expected uplink 100 Mbps, got %s", retrievedPolicy.Ambr.Uplink)
	}

	if len(retrievedPolicy.NetworkRules) != 2 {
		t.Fatalf("expected 2 network rules, got %d", len(retrievedPolicy.NetworkRules))
	}

	rule1Found := false
	rule2Found := false

	for _, r := range retrievedPolicy.NetworkRules {
		if r.Description == "rule-1" {
			rule1Found = true

			if r.Direction != smf.DirectionUplink {
				t.Fatalf("rule-1 expected direction uplink, got %s", r.Direction)
			}

			if r.RemotePrefix == nil || *r.RemotePrefix != "192.168.0.0/24" {
				t.Fatalf("rule-1 expected prefix 192.168.0.0/24, got %v", r.RemotePrefix)
			}

			if r.Protocol != 6 {
				t.Fatalf("rule-1 expected protocol 6, got %d", r.Protocol)
			}

			if r.PortLow != 80 {
				t.Fatalf("rule-1 expected port low 80, got %d", r.PortLow)
			}

			if r.PortHigh != 443 {
				t.Fatalf("rule-1 expected port high 443, got %d", r.PortHigh)
			}

			if r.Action != "allow" {
				t.Fatalf("rule-1 expected action allow, got %s", r.Action)
			}

			if r.Precedence != 1 {
				t.Fatalf("rule-1 expected precedence 1, got %d", r.Precedence)
			}
		}

		if r.Description == "rule-2" {
			rule2Found = true

			if r.Direction != smf.DirectionDownlink {
				t.Fatalf("rule-2 expected direction downlink, got %s", r.Direction)
			}

			if r.RemotePrefix == nil || *r.RemotePrefix != "10.0.0.0/8" {
				t.Fatalf("rule-2 expected prefix 10.0.0.0/8, got %v", r.RemotePrefix)
			}

			if r.Protocol != 17 {
				t.Fatalf("rule-2 expected protocol 17, got %d", r.Protocol)
			}

			if r.PortLow != 5060 {
				t.Fatalf("rule-2 expected port low 5060, got %d", r.PortLow)
			}

			if r.PortHigh != 5060 {
				t.Fatalf("rule-2 expected port high 5060, got %d", r.PortHigh)
			}

			if r.Action != "deny" {
				t.Fatalf("rule-2 expected action deny, got %s", r.Action)
			}

			if r.Precedence != 2 {
				t.Fatalf("rule-2 expected precedence 2, got %d", r.Precedence)
			}
		}
	}

	if !rule1Found {
		t.Fatalf("rule-1 not found in network rules")
	}

	if !rule2Found {
		t.Fatalf("rule-2 not found in network rules")
	}
}

func TestGetSessionPolicy_NoNetworkRules(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test database: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("couldn't close database: %s", err)
		}
	}()

	ctx := context.Background()

	testDN := &db.DataNetwork{Name: "test-dnn-2", IPPool: "10.2.0.0/24"}
	if err := database.CreateDataNetwork(ctx, testDN); err != nil {
		t.Fatalf("couldn't create test data network: %s", err)
	}

	testDataNetwork, err := database.GetDataNetwork(ctx, "test-dnn-2")
	if err != nil {
		t.Fatalf("couldn't get test data network: %s", err)
	}

	testProfile := &db.Profile{Name: "test-profile-2", UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps"}
	if err := database.CreateProfile(ctx, testProfile); err != nil {
		t.Fatalf("couldn't create test profile: %s", err)
	}

	createdProfile, err := database.GetProfile(ctx, "test-profile-2")
	if err != nil {
		t.Fatalf("couldn't get test profile: %s", err)
	}

	policy := &db.Policy{
		Name:                "test-policy-no-rules",
		SessionAmbrUplink:   "50 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       testDataNetwork.ID,
		ProfileID:           createdProfile.ID,
		SliceID:             1,
	}

	err = database.CreatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("couldn't create test policy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(ctx, "test-policy-no-rules")
	if err != nil {
		t.Fatalf("couldn't get created policy: %s", err)
	}

	subscriber := &db.Subscriber{
		Imsi:           "310410000000002",
		SequenceNumber: "000000000002",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      createdPolicy.ProfileID,
	}

	err = database.CreateSubscriber(ctx, subscriber)
	if err != nil {
		t.Fatalf("couldn't create subscriber: %s", err)
	}

	adapter := runtime.NewSMFDBAdapter(database)

	snssai := &models.Snssai{Sst: db.InitialSliceSst, Sd: db.InitialSliceSd}

	retrievedPolicy, err := adapter.GetSessionPolicy(ctx, "310410000000002", snssai, "test-dnn-2")
	if err != nil {
		t.Fatalf("GetSessionPolicy failed: %v", err)
	}

	if retrievedPolicy == nil {
		t.Fatalf("expected non-nil policy")
	}

	if len(retrievedPolicy.NetworkRules) != 0 {
		t.Fatalf("expected 0 network rules, got %d", len(retrievedPolicy.NetworkRules))
	}
}
