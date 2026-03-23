// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestNetworkRulesCreateGetUpdate(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dataNetwork, err := database.GetDataNetwork(context.Background(), db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	newPolicy := &db.Policy{
		Name:            "test-policy",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), newPolicy)
	if err != nil {
		t.Fatalf("Couldn't create test policy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(context.Background(), newPolicy.Name)
	if err != nil {
		t.Fatalf("Couldn't get created policy: %s", err)
	}

	rule := &db.NetworkRule{
		PolicyID:     int64(createdPolicy.ID),
		Description:  "test-rule-1",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "permit",
		Precedence:   1,
	}

	id, err := database.CreateNetworkRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("Couldn't complete CreateNetworkRule: %s", err)
	}

	if id == 0 {
		t.Fatalf("Expected non-zero ID from CreateNetworkRule")
	}

	retrieved, err := database.GetNetworkRule(context.Background(), id)
	if err != nil {
		t.Fatalf("Couldn't complete GetNetworkRule: %s", err)
	}

	if retrieved.Description != rule.Description {
		t.Fatalf("Retrieved rule description %q doesn't match created rule description %q", retrieved.Description, rule.Description)
	}

	if retrieved.Direction != rule.Direction {
		t.Fatalf("Retrieved rule direction %q doesn't match created rule direction %q", retrieved.Direction, rule.Direction)
	}

	if retrieved.Action != rule.Action {
		t.Fatalf("Retrieved rule action %q doesn't match created rule action %q", retrieved.Action, rule.Action)
	}

	if retrieved.PolicyID != int64(createdPolicy.ID) {
		t.Fatalf("Retrieved rule policy_id %d doesn't match expected policy_id %d", retrieved.PolicyID, createdPolicy.ID)
	}

	retrieved.Direction = "downlink"
	retrieved.Action = "deny"

	err = database.UpdateNetworkRule(context.Background(), retrieved)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateNetworkRule: %s", err)
	}

	updated, err := database.GetNetworkRule(context.Background(), id)
	if err != nil {
		t.Fatalf("Couldn't complete GetNetworkRule after update: %s", err)
	}

	if updated.Direction != "downlink" {
		t.Fatalf("Updated rule direction %q doesn't match expected value %q", updated.Direction, "downlink")
	}

	if updated.Action != "deny" {
		t.Fatalf("Updated rule action %q doesn't match expected value %q", updated.Action, "deny")
	}
}

func TestNetworkRulesDelete(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dataNetwork, err := database.GetDataNetwork(context.Background(), db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	newPolicy := &db.Policy{
		Name:            "test-policy-delete",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), newPolicy)
	if err != nil {
		t.Fatalf("Couldn't create test policy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(context.Background(), newPolicy.Name)
	if err != nil {
		t.Fatalf("Couldn't get created policy: %s", err)
	}

	rule := &db.NetworkRule{
		PolicyID:     int64(createdPolicy.ID),
		Description:  "test-rule-delete",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "permit",
		Precedence:   1,
	}

	id, err := database.CreateNetworkRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("Couldn't complete CreateNetworkRule: %s", err)
	}

	err = database.DeleteNetworkRule(context.Background(), id)
	if err != nil {
		t.Fatalf("Couldn't complete DeleteNetworkRule: %s", err)
	}

	_, err = database.GetNetworkRule(context.Background(), id)
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound after delete, got: %s", err)
	}
}

func TestNetworkRulesDuplicatePrecedencePerPolicy(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dataNetwork, err := database.GetDataNetwork(context.Background(), db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	newPolicy := &db.Policy{
		Name:            "test-policy-precedence-unique",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), newPolicy)
	if err != nil {
		t.Fatalf("Couldn't create test policy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(context.Background(), newPolicy.Name)
	if err != nil {
		t.Fatalf("Couldn't get created policy: %s", err)
	}

	_, err = database.CreateNetworkRule(context.Background(), &db.NetworkRule{
		PolicyID:    int64(createdPolicy.ID),
		Description: "rule-precedence-100",
		Direction:   "uplink",
		Protocol:    6,
		PortLow:     80,
		PortHigh:    80,
		Action:      "permit",
		Precedence:  100,
	})
	if err != nil {
		t.Fatalf("Couldn't create first rule: %s", err)
	}

	_, err = database.CreateNetworkRule(context.Background(), &db.NetworkRule{
		PolicyID:    int64(createdPolicy.ID),
		Description: "rule-precedence-100-duplicate-same-direction",
		Direction:   "uplink",
		Protocol:    6,
		PortLow:     443,
		PortHigh:    443,
		Action:      "permit",
		Precedence:  100,
	})
	if err != db.ErrAlreadyExists {
		t.Fatalf("Expected ErrAlreadyExists for duplicate (policy_id, precedence, direction), got: %v", err)
	}
}

func TestNetworkRulesDuplicatePrecedenceDifferentPoliciesAllowed(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dataNetwork, err := database.GetDataNetwork(context.Background(), db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	policy1 := &db.Policy{
		Name:            "test-policy-prec-1",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}
	if err := database.CreatePolicy(context.Background(), policy1); err != nil {
		t.Fatalf("Couldn't create policy 1: %s", err)
	}

	createdPolicy1, err := database.GetPolicy(context.Background(), policy1.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 1: %s", err)
	}

	policy2 := &db.Policy{
		Name:            "test-policy-prec-2",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}
	if err := database.CreatePolicy(context.Background(), policy2); err != nil {
		t.Fatalf("Couldn't create policy 2: %s", err)
	}

	createdPolicy2, err := database.GetPolicy(context.Background(), policy2.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 2: %s", err)
	}

	_, err = database.CreateNetworkRule(context.Background(), &db.NetworkRule{
		PolicyID:    int64(createdPolicy1.ID),
		Description: "rule-p1",
		Direction:   "uplink",
		Protocol:    6,
		Action:      "permit",
		Precedence:  100,
	})
	if err != nil {
		t.Fatalf("Couldn't create rule for policy 1: %s", err)
	}

	_, err = database.CreateNetworkRule(context.Background(), &db.NetworkRule{
		PolicyID:    int64(createdPolicy2.ID),
		Description: "rule-p2",
		Direction:   "uplink",
		Protocol:    6,
		Action:      "permit",
		Precedence:  100,
	})
	if err != nil {
		t.Fatalf("Same precedence value in a different policy should be allowed, got: %s", err)
	}
}

func TestNetworkRulesDuplicateNamePerPolicy(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dataNetwork, err := database.GetDataNetwork(context.Background(), db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	newPolicy := &db.Policy{
		Name:            "test-policy-unique",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), newPolicy)
	if err != nil {
		t.Fatalf("Couldn't create test policy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(context.Background(), newPolicy.Name)
	if err != nil {
		t.Fatalf("Couldn't get created policy: %s", err)
	}

	rule := &db.NetworkRule{
		PolicyID:     int64(createdPolicy.ID),
		Description:  "duplicate-rule",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "permit",
		Precedence:   1,
	}

	_, err = database.CreateNetworkRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("Couldn't create first rule: %s", err)
	}

	_, err = database.CreateNetworkRule(context.Background(), rule)
	if err != db.ErrAlreadyExists {
		t.Fatalf("Expected ErrAlreadyExists for duplicate (policy_id, name), got: %s", err)
	}
}

func TestNetworkRulesDifferentPoliciesSameName(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dataNetwork, err := database.GetDataNetwork(context.Background(), db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	policy1 := &db.Policy{
		Name:            "test-policy-1",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), policy1)
	if err != nil {
		t.Fatalf("Couldn't create policy 1: %s", err)
	}

	createdPolicy1, err := database.GetPolicy(context.Background(), policy1.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 1: %s", err)
	}

	policy2 := &db.Policy{
		Name:            "test-policy-2",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), policy2)
	if err != nil {
		t.Fatalf("Couldn't create policy 2: %s", err)
	}

	createdPolicy2, err := database.GetPolicy(context.Background(), policy2.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 2: %s", err)
	}

	rule1 := &db.NetworkRule{
		PolicyID:     int64(createdPolicy1.ID),
		Description:  "same-rule-name",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "permit",
		Precedence:   1,
	}

	_, err = database.CreateNetworkRule(context.Background(), rule1)
	if err != nil {
		t.Fatalf("Couldn't create rule for policy 1: %s", err)
	}

	rule2 := &db.NetworkRule{
		PolicyID:     int64(createdPolicy2.ID),
		Description:  "same-rule-name",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "permit",
		Precedence:   1,
	}

	_, err = database.CreateNetworkRule(context.Background(), rule2)
	if err != nil {
		t.Fatalf("Couldn't create rule with same name in different policy: %s", err)
	}
}

func TestListRulesForPolicy(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dataNetwork, err := database.GetDataNetwork(context.Background(), db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	policy1 := &db.Policy{
		Name:            "test-policy-with-rules",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), policy1)
	if err != nil {
		t.Fatalf("Couldn't create policy 1: %s", err)
	}

	createdPolicy1, err := database.GetPolicy(context.Background(), policy1.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 1: %s", err)
	}

	policy2 := &db.Policy{
		Name:            "test-policy-no-rules",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), policy2)
	if err != nil {
		t.Fatalf("Couldn't create policy 2: %s", err)
	}

	createdPolicy2, err := database.GetPolicy(context.Background(), policy2.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 2: %s", err)
	}

	var ruleIDs []int64

	for i := 1; i <= 3; i++ {
		rule := &db.NetworkRule{
			PolicyID:     int64(createdPolicy1.ID),
			Description:  "rule-for-listing-" + string(rune('0'+i)),
			Direction:    "uplink",
			RemotePrefix: nil,
			Protocol:     6,
			PortLow:      80,
			PortHigh:     443,
			Action:       "permit",
			Precedence:   int32(i),
		}

		ruleID, err := database.CreateNetworkRule(context.Background(), rule)
		if err != nil {
			t.Fatalf("Couldn't create test rule %d: %s", i, err)
		}

		ruleIDs = append(ruleIDs, ruleID)
	}

	retrievedRules1, err := database.ListRulesForPolicy(context.Background(), int64(createdPolicy1.ID))
	if err != nil {
		t.Fatalf("Couldn't list rules for policy 1: %s", err)
	}

	if len(retrievedRules1) != 3 {
		t.Fatalf("Expected 3 rules for policy 1, got %d", len(retrievedRules1))
	}

	for i, rule := range retrievedRules1 {
		found := false

		for _, id := range ruleIDs {
			if rule.ID == id {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf("Rule %d not found in expected rule IDs", i)
		}
	}

	retrievedRules2, err := database.ListRulesForPolicy(context.Background(), int64(createdPolicy2.ID))
	if err != nil {
		t.Fatalf("Couldn't list rules for policy 2: %s", err)
	}

	if len(retrievedRules2) != 0 {
		t.Fatalf("Expected 0 rules for policy 2, got %d", len(retrievedRules2))
	}
}

func setupTestDB(t *testing.T) *db.Database {
	tempDir := t.TempDir()

	dbInstance, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	t.Cleanup(func() {
		if err := dbInstance.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	})

	return dbInstance
}

func createTestPolicy(t *testing.T, dbInstance *db.Database) *db.Policy {
	dataNetwork, err := dbInstance.GetDataNetwork(context.Background(), db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	policy := &db.Policy{
		Name:            "test-policy-" + t.Name(),
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   dataNetwork.ID,
	}

	err = dbInstance.CreatePolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("Couldn't create test policy: %s", err)
	}

	createdPolicy, err := dbInstance.GetPolicy(context.Background(), policy.Name)
	if err != nil {
		t.Fatalf("Couldn't get created policy: %s", err)
	}

	return createdPolicy
}

func TestListRulesForPolicy_Ordering(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	policy := createTestPolicy(t, dbInstance)

	rules := []struct {
		name       string
		precedence int32
	}{
		{"rule-300", 300},
		{"rule-100", 100},
		{"rule-200", 200},
	}

	ids := make([]int64, len(rules))
	for i, tc := range rules {
		id, err := dbInstance.CreateNetworkRule(ctx, &db.NetworkRule{
			PolicyID:    int64(policy.ID),
			Description: tc.name,
			Direction:   "uplink",
			Protocol:    6,
			PortLow:     80,
			PortHigh:    80,
			Action:      "allow",
			Precedence:  tc.precedence,
		})
		if err != nil {
			t.Fatalf("CreateNetworkRule(%s): %v", tc.name, err)
		}

		ids[i] = id
	}

	got, err := dbInstance.ListRulesForPolicy(ctx, int64(policy.ID))
	if err != nil {
		t.Fatalf("ListRulesForPolicy: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(got))
	}

	wantPrecedences := []int32{100, 200, 300}
	for i, r := range got {
		if r.Precedence != wantPrecedences[i] {
			t.Errorf("rule[%d]: want precedence %d, got %d", i, wantPrecedences[i], r.Precedence)
		}
	}
}

func TestReorderRulesForPolicy(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	policy := createTestPolicy(t, dbInstance)

	var ids [3]int64

	for i, name := range []string{"rule-a", "rule-b", "rule-c"} {
		id, err := dbInstance.CreateNetworkRule(ctx, &db.NetworkRule{
			PolicyID:    int64(policy.ID),
			Description: name,
			Direction:   "uplink",
			Protocol:    6,
			Action:      "allow",
			Precedence:  int32((i + 1) * 100),
		})
		if err != nil {
			t.Fatalf("CreateNetworkRule(%s): %v", name, err)
		}

		ids[i] = id
	}

	if err := dbInstance.ReorderRulesForPolicy(ctx, int64(policy.ID), ids[2], 0, "uplink"); err != nil {
		t.Fatalf("ReorderRulesForPolicy: %v", err)
	}

	got, err := dbInstance.ListRulesForPolicy(ctx, int64(policy.ID))
	if err != nil {
		t.Fatalf("ListRulesForPolicy after reorder: %v", err)
	}

	wantIDs := []int64{ids[2], ids[0], ids[1]}
	for i, r := range got {
		if r.ID != wantIDs[i] {
			t.Errorf("rule[%d]: want id %d, got %d", i, wantIDs[i], r.ID)
		}

		if r.Precedence != int32((i+1)*100)+1 {
			t.Errorf("rule[%d]: want precedence %d, got %d", i, (i+1)*100+1, r.Precedence)
		}
	}

	if err := dbInstance.ReorderRulesForPolicy(ctx, int64(policy.ID), ids[0], 2, "uplink"); err != nil {
		t.Fatalf("ReorderRulesForPolicy: %v", err)
	}

	got, err = dbInstance.ListRulesForPolicy(ctx, int64(policy.ID))
	if err != nil {
		t.Fatalf("ListRulesForPolicy after reorder: %v", err)
	}

	wantIDs = []int64{ids[2], ids[1], ids[0]}
	for i, r := range got {
		if r.ID != wantIDs[i] {
			t.Errorf("rule[%d]: want id %d, got %d", i, wantIDs[i], r.ID)
		}

		if r.Precedence != int32((i+1)*100) {
			t.Errorf("rule[%d]: want precedence %d, got %d", i, (i+1)*100, r.Precedence)
		}
	}
}
