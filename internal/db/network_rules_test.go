// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// createPolicyDeps creates a profile and a network slice and returns their IDs.
// Used to satisfy the FK constraints on policies.profile_id / slice_id.
func createPolicyDeps(t *testing.T, database *db.Database, suffix string) (profileID string, sliceID string) {
	t.Helper()

	profile := &db.Profile{
		Name:           "test-profile-" + suffix,
		UeAmbrUplink:   "200 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}
	if err := database.CreateProfile(context.Background(), profile); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	createdProfile, err := database.GetProfile(context.Background(), profile.Name)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	slice := &db.NetworkSlice{Name: "test-slice-" + suffix, Sst: 1}
	if err := database.CreateNetworkSlice(context.Background(), slice); err != nil {
		t.Fatalf("CreateNetworkSlice: %s", err)
	}

	createdSlice, err := database.GetNetworkSlice(context.Background(), slice.Name)
	if err != nil {
		t.Fatalf("GetNetworkSlice: %s", err)
	}

	return createdProfile.ID, createdSlice.ID
}

func TestNetworkRulesCreateGetUpdate(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	testDN := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN); err != nil {
		t.Fatalf("Couldnt create test data network: %s", err)
	}

	dataNetwork, err := database.GetDataNetwork(context.Background(), "test-dnn")
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "create-get-update")

	newPolicy := &db.Policy{
		Name:                "test-policy",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
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
		PolicyID:     createdPolicy.ID,
		Description:  "test-rule-1",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "allow",
		Precedence:   1,
	}

	id, err := database.CreateNetworkRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("Couldn't complete CreateNetworkRule: %s", err)
	}

	if id == "" {
		t.Fatalf("Expected non-empty ID from CreateNetworkRule")
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

	if retrieved.PolicyID != createdPolicy.ID {
		t.Fatalf("Retrieved rule policy_id %s doesn't match expected policy_id %s", retrieved.PolicyID, createdPolicy.ID)
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

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	testDN := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN); err != nil {
		t.Fatalf("Couldnt create test data network: %s", err)
	}

	dataNetwork, err := database.GetDataNetwork(context.Background(), "test-dnn")
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "delete")

	newPolicy := &db.Policy{
		Name:                "test-policy-delete",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
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
		PolicyID:     createdPolicy.ID,
		Description:  "test-rule-delete",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "allow",
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

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	testDN := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN); err != nil {
		t.Fatalf("Couldnt create test data network: %s", err)
	}

	dataNetwork, err := database.GetDataNetwork(context.Background(), "test-dnn")
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "precedence-unique")

	newPolicy := &db.Policy{
		Name:                "test-policy-precedence-unique",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
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
		PolicyID:    createdPolicy.ID,
		Description: "rule-precedence-100",
		Direction:   "uplink",
		Protocol:    6,
		PortLow:     80,
		PortHigh:    80,
		Action:      "allow",
		Precedence:  100,
	})
	if err != nil {
		t.Fatalf("Couldn't create first rule: %s", err)
	}

	_, err = database.CreateNetworkRule(context.Background(), &db.NetworkRule{
		PolicyID:    createdPolicy.ID,
		Description: "rule-precedence-100-duplicate-same-direction",
		Direction:   "uplink",
		Protocol:    6,
		PortLow:     443,
		PortHigh:    443,
		Action:      "allow",
		Precedence:  100,
	})
	if err != db.ErrAlreadyExists {
		t.Fatalf("Expected ErrAlreadyExists for duplicate (policy_id, precedence, direction), got: %v", err)
	}
}

func TestNetworkRulesDuplicatePrecedenceDifferentPoliciesAllowed(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	testDN := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN); err != nil {
		t.Fatalf("Couldnt create test data network: %s", err)
	}

	dataNetwork, err := database.GetDataNetwork(context.Background(), "test-dnn")
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "prec")

	policy1 := &db.Policy{
		Name:                "test-policy-prec-1",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
	}
	if err := database.CreatePolicy(context.Background(), policy1); err != nil {
		t.Fatalf("Couldn't create policy 1: %s", err)
	}

	createdPolicy1, err := database.GetPolicy(context.Background(), policy1.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 1: %s", err)
	}

	testDN2 := &db.DataNetwork{Name: "test-dnn-2", IPv4Pool: "10.2.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN2); err != nil {
		t.Fatalf("Couldnt create second test data network: %s", err)
	}

	dataNetwork2, err := database.GetDataNetwork(context.Background(), "test-dnn-2")
	if err != nil {
		t.Fatalf("Couldn't get second data network: %s", err)
	}

	policy2 := &db.Policy{
		Name:                "test-policy-prec-2",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork2.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
	}
	if err := database.CreatePolicy(context.Background(), policy2); err != nil {
		t.Fatalf("Couldn't create policy 2: %s", err)
	}

	createdPolicy2, err := database.GetPolicy(context.Background(), policy2.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 2: %s", err)
	}

	_, err = database.CreateNetworkRule(context.Background(), &db.NetworkRule{
		PolicyID:    createdPolicy1.ID,
		Description: "rule-p1",
		Direction:   "uplink",
		Protocol:    6,
		Action:      "allow",
		Precedence:  100,
	})
	if err != nil {
		t.Fatalf("Couldn't create rule for policy 1: %s", err)
	}

	_, err = database.CreateNetworkRule(context.Background(), &db.NetworkRule{
		PolicyID:    createdPolicy2.ID,
		Description: "rule-p2",
		Direction:   "uplink",
		Protocol:    6,
		Action:      "allow",
		Precedence:  100,
	})
	if err != nil {
		t.Fatalf("Same precedence value in a different policy should be allowed, got: %s", err)
	}
}

func TestNetworkRulesDuplicateNamePerPolicy(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	testDN := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN); err != nil {
		t.Fatalf("Couldnt create test data network: %s", err)
	}

	dataNetwork, err := database.GetDataNetwork(context.Background(), "test-dnn")
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "name-unique")

	newPolicy := &db.Policy{
		Name:                "test-policy-unique",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
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
		PolicyID:     createdPolicy.ID,
		Description:  "duplicate-rule",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "allow",
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

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	testDN := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN); err != nil {
		t.Fatalf("Couldnt create test data network: %s", err)
	}

	dataNetwork, err := database.GetDataNetwork(context.Background(), "test-dnn")
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "diff-pol-same-name")

	policy1 := &db.Policy{
		Name:                "test-policy-1",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
	}

	err = database.CreatePolicy(context.Background(), policy1)
	if err != nil {
		t.Fatalf("Couldn't create policy 1: %s", err)
	}

	createdPolicy1, err := database.GetPolicy(context.Background(), policy1.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 1: %s", err)
	}

	testDN2 := &db.DataNetwork{Name: "test-dnn-2", IPv4Pool: "10.2.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN2); err != nil {
		t.Fatalf("Couldnt create second test data network: %s", err)
	}

	dataNetwork2, err := database.GetDataNetwork(context.Background(), "test-dnn-2")
	if err != nil {
		t.Fatalf("Couldn't get second data network: %s", err)
	}

	policy2 := &db.Policy{
		Name:                "test-policy-2",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork2.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
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
		PolicyID:     createdPolicy1.ID,
		Description:  "same-rule-name",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "allow",
		Precedence:   1,
	}

	_, err = database.CreateNetworkRule(context.Background(), rule1)
	if err != nil {
		t.Fatalf("Couldn't create rule for policy 1: %s", err)
	}

	rule2 := &db.NetworkRule{
		PolicyID:     createdPolicy2.ID,
		Description:  "same-rule-name",
		Direction:    "uplink",
		RemotePrefix: nil,
		Protocol:     6,
		PortLow:      80,
		PortHigh:     443,
		Action:       "allow",
		Precedence:   1,
	}

	_, err = database.CreateNetworkRule(context.Background(), rule2)
	if err != nil {
		t.Fatalf("Couldn't create rule with same name in different policy: %s", err)
	}
}

func TestListRulesForPolicy(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	testDN := &db.DataNetwork{Name: "test-dnn", IPv4Pool: "10.1.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN); err != nil {
		t.Fatalf("Couldnt create test data network: %s", err)
	}

	dataNetwork, err := database.GetDataNetwork(context.Background(), "test-dnn")
	if err != nil {
		t.Fatalf("Couldn't get initial data network: %s", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "list-rules")

	policy1 := &db.Policy{
		Name:                "test-policy-with-rules",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
	}

	err = database.CreatePolicy(context.Background(), policy1)
	if err != nil {
		t.Fatalf("Couldn't create policy 1: %s", err)
	}

	createdPolicy1, err := database.GetPolicy(context.Background(), policy1.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 1: %s", err)
	}

	testDN2 := &db.DataNetwork{Name: "test-dnn-2", IPv4Pool: "10.2.0.0/24"}
	if err := database.CreateDataNetwork(context.Background(), testDN2); err != nil {
		t.Fatalf("Couldnt create second test data network: %s", err)
	}

	dataNetwork2, err := database.GetDataNetwork(context.Background(), "test-dnn-2")
	if err != nil {
		t.Fatalf("Couldn't get second data network: %s", err)
	}

	policy2 := &db.Policy{
		Name:                "test-policy-no-rules",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork2.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
	}

	err = database.CreatePolicy(context.Background(), policy2)
	if err != nil {
		t.Fatalf("Couldn't create policy 2: %s", err)
	}

	createdPolicy2, err := database.GetPolicy(context.Background(), policy2.Name)
	if err != nil {
		t.Fatalf("Couldn't get policy 2: %s", err)
	}

	var ruleIDs []string

	for i := 1; i <= 3; i++ {
		rule := &db.NetworkRule{
			PolicyID:     createdPolicy1.ID,
			Description:  "rule-for-listing-" + string(rune('0'+i)),
			Direction:    "uplink",
			RemotePrefix: nil,
			Protocol:     6,
			PortLow:      80,
			PortHigh:     443,
			Action:       "allow",
			Precedence:   int32(i),
		}

		ruleID, err := database.CreateNetworkRule(context.Background(), rule)
		if err != nil {
			t.Fatalf("Couldn't create test rule %d: %s", i, err)
		}

		ruleIDs = append(ruleIDs, ruleID)
	}

	retrievedRules1, err := database.ListRulesForPolicy(context.Background(), createdPolicy1.ID)
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

	retrievedRules2, err := database.ListRulesForPolicy(context.Background(), createdPolicy2.ID)
	if err != nil {
		t.Fatalf("Couldn't list rules for policy 2: %s", err)
	}

	if len(retrievedRules2) != 0 {
		t.Fatalf("Expected 0 rules for policy 2, got %d", len(retrievedRules2))
	}
}

func setupTestDB(t *testing.T) *db.Database {
	tempDir := t.TempDir()

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
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
	testDN := &db.DataNetwork{Name: "test-dnn-" + t.Name(), IPv4Pool: "10.3.0.0/24"}
	if err := dbInstance.CreateDataNetwork(context.Background(), testDN); err != nil {
		t.Fatalf("Couldn't create test data network: %s", err)
	}

	dataNetwork, err := dbInstance.GetDataNetwork(context.Background(), testDN.Name)
	if err != nil {
		t.Fatalf("Couldn't get test data network: %s", err)
	}

	profileID, sliceID := createPolicyDeps(t, dbInstance, t.Name())

	policy := &db.Policy{
		Name:                "test-policy-" + t.Name(),
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dataNetwork.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
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

	ids := make([]string, len(rules))
	for i, tc := range rules {
		id, err := dbInstance.CreateNetworkRule(ctx, &db.NetworkRule{
			PolicyID:    policy.ID,
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

	got, err := dbInstance.ListRulesForPolicy(ctx, policy.ID)
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

	var ids [3]string

	for i, name := range []string{"rule-a", "rule-b", "rule-c"} {
		id, err := dbInstance.CreateNetworkRule(ctx, &db.NetworkRule{
			PolicyID:    policy.ID,
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

	if err := dbInstance.ReorderRulesForPolicy(ctx, policy.ID, ids[2], 0, "uplink"); err != nil {
		t.Fatalf("ReorderRulesForPolicy: %v", err)
	}

	got, err := dbInstance.ListRulesForPolicy(ctx, policy.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy after reorder: %v", err)
	}

	wantIDs := []string{ids[2], ids[0], ids[1]}
	for i, r := range got {
		if r.ID != wantIDs[i] {
			t.Errorf("rule[%d]: want id %s, got %s", i, wantIDs[i], r.ID)
		}

		if r.Precedence != int32((i+1)*100)+1 {
			t.Errorf("rule[%d]: want precedence %d, got %d", i, (i+1)*100+1, r.Precedence)
		}
	}

	if err := dbInstance.ReorderRulesForPolicy(ctx, policy.ID, ids[0], 2, "uplink"); err != nil {
		t.Fatalf("ReorderRulesForPolicy: %v", err)
	}

	got, err = dbInstance.ListRulesForPolicy(ctx, policy.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy after reorder: %v", err)
	}

	wantIDs = []string{ids[2], ids[1], ids[0]}
	for i, r := range got {
		if r.ID != wantIDs[i] {
			t.Errorf("rule[%d]: want id %s, got %s", i, wantIDs[i], r.ID)
		}

		if r.Precedence != int32((i+1)*100) {
			t.Errorf("rule[%d]: want precedence %d, got %d", i, (i+1)*100, r.Precedence)
		}
	}
}

// TestUpdatePolicyWithRules_AllZeroRule tests that a rule with all-zero
// numeric fields (protocol=0, port_low=0, port_high=0) is correctly stored
// and retrieved by UpdatePolicyWithRules. This exercises the specific
// "allow all" rule shape that was reported as being silently dropped.
func TestUpdatePolicyWithRules_AllZeroRule_NoRaft(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	policy := createTestPolicy(t, dbInstance)

	// Start with 4 non-zero rules.
	initial := &db.PolicyRulesInput{
		Uplink: []db.PolicyRuleInput{
			{Description: "rule-1", Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			{Description: "rule-2", Protocol: 6, PortLow: 443, PortHigh: 443, Action: "allow"},
			{Description: "rule-3", Protocol: 17, PortLow: 53, PortHigh: 53, Action: "allow"},
			{Description: "rule-4", Protocol: 1, PortLow: 0, PortHigh: 0, Action: "deny"},
		},
	}

	if err := dbInstance.UpdatePolicyWithRules(ctx, policy, initial); err != nil {
		t.Fatalf("UpdatePolicyWithRules (initial): %v", err)
	}

	rules, err := dbInstance.ListRulesForPolicy(ctx, policy.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy after initial update: %v", err)
	}

	if len(rules) != 4 {
		t.Fatalf("expected 4 rules after initial update, got %d", len(rules))
	}

	// Now update with 5 rules where the 5th is an "allow all"
	// (protocol=0, port_low=0, port_high=0, no remote_prefix).
	updated := &db.PolicyRulesInput{
		Uplink: []db.PolicyRuleInput{
			{Description: "rule-1", Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			{Description: "rule-2", Protocol: 6, PortLow: 443, PortHigh: 443, Action: "allow"},
			{Description: "rule-3", Protocol: 17, PortLow: 53, PortHigh: 53, Action: "allow"},
			{Description: "rule-4", Protocol: 1, PortLow: 0, PortHigh: 0, Action: "deny"},
			{Description: "allow all", Protocol: 0, PortLow: 0, PortHigh: 0, Action: "allow"},
		},
	}

	if err := dbInstance.UpdatePolicyWithRules(ctx, policy, updated); err != nil {
		t.Fatalf("UpdatePolicyWithRules (with allow-all): %v", err)
	}

	rules, err = dbInstance.ListRulesForPolicy(ctx, policy.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy after update with allow-all: %v", err)
	}

	if len(rules) != 5 {
		t.Fatalf("expected 5 rules after update, got %d (allow-all rule may have been dropped)", len(rules))
	}

	found := false

	for _, r := range rules {
		if r.Description == "allow all" && r.Protocol == 0 && r.PortLow == 0 && r.PortHigh == 0 {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("allow-all rule (protocol=0, port_low=0, port_high=0) not found in stored rules")
	}
}

// TestUpdatePolicyWithRules_AllZeroRule_Raft is the Raft-path analogue of
// TestUpdatePolicyWithRules_AllZeroRule_NoRaft. It verifies the changeset
// capture and Raft proposal succeed for the all-zero rule shape.
func TestUpdatePolicyWithRules_AllZeroRule_Raft(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "data")

	database, err := db.NewDatabase(ctx, dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	testDN := &db.DataNetwork{Name: "allzero-dn", IPv4Pool: "10.77.0.0/24"}
	if err := database.CreateDataNetwork(ctx, testDN); err != nil {
		t.Fatalf("CreateDataNetwork: %v", err)
	}

	dn, err := database.GetDataNetwork(ctx, testDN.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork: %v", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "allzero-raft")

	policy := &db.Policy{
		Name:                "allzero-raft-policy",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dn.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
	}

	if err := database.CreatePolicy(ctx, policy); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	created, err := database.GetPolicy(ctx, policy.Name)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}

	// Populate 4 initial rules.
	initial := &db.PolicyRulesInput{
		Uplink: []db.PolicyRuleInput{
			{Description: "rule-1", Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			{Description: "rule-2", Protocol: 6, PortLow: 443, PortHigh: 443, Action: "allow"},
			{Description: "rule-3", Protocol: 17, PortLow: 53, PortHigh: 53, Action: "allow"},
			{Description: "rule-4", Protocol: 1, PortLow: 0, PortHigh: 0, Action: "deny"},
		},
	}

	if err := database.UpdatePolicyWithRules(ctx, created, initial); err != nil {
		t.Fatalf("UpdatePolicyWithRules (initial): %v", err)
	}

	rules, err := database.ListRulesForPolicy(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy after initial: %v", err)
	}

	if len(rules) != 4 {
		t.Fatalf("expected 4 rules after initial update, got %d", len(rules))
	}

	// Now update with the all-zero "allow all" rule added as the 5th.
	withAllZero := &db.PolicyRulesInput{
		Uplink: []db.PolicyRuleInput{
			{Description: "rule-1", Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			{Description: "rule-2", Protocol: 6, PortLow: 443, PortHigh: 443, Action: "allow"},
			{Description: "rule-3", Protocol: 17, PortLow: 53, PortHigh: 53, Action: "allow"},
			{Description: "rule-4", Protocol: 1, PortLow: 0, PortHigh: 0, Action: "deny"},
			{Description: "allow all", Protocol: 0, PortLow: 0, PortHigh: 0, Action: "allow"},
		},
	}

	if err := database.UpdatePolicyWithRules(ctx, created, withAllZero); err != nil {
		t.Fatalf("UpdatePolicyWithRules (with allow-all): %v", err)
	}

	rules, err = database.ListRulesForPolicy(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy after allow-all update: %v", err)
	}

	if len(rules) != 5 {
		t.Fatalf("expected 5 rules after update, got %d (allow-all rule may have been dropped by changeset path)", len(rules))
	}

	found := false

	for _, r := range rules {
		if r.Description == "allow all" && r.Protocol == 0 && r.PortLow == 0 && r.PortHigh == 0 {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("allow-all rule (protocol=0, port_low=0, port_high=0) not found after Raft update")
	}
}

// TestUpdatePolicyWithRules_Raft_SurviveRestart verifies that rules written
// through the Raft changeset path persist across a full database close/reopen.
// This is the production code path (single-node Raft). The earlier API-level
// restart test uses NewDatabaseWithoutRaft, which bypasses changeset capture.
func TestUpdatePolicyWithRules_Raft_SurviveRestart(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "data")

	database, err := db.NewDatabase(ctx, dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	// Create prerequisite entities.
	testDN := &db.DataNetwork{Name: "raft-dn", IPv4Pool: "10.99.0.0/24"}
	if err := database.CreateDataNetwork(ctx, testDN); err != nil {
		t.Fatalf("CreateDataNetwork: %v", err)
	}

	dn, err := database.GetDataNetwork(ctx, testDN.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork: %v", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "raft-restart")

	policy := &db.Policy{
		Name:                "raft-restart-policy",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dn.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
	}

	if err := database.CreatePolicy(ctx, policy); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	created, err := database.GetPolicy(ctx, policy.Name)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}

	cidr := "2001:db8::/64"
	rules := &db.PolicyRulesInput{
		Uplink: []db.PolicyRuleInput{{
			Description:  "allow-ipv6",
			RemotePrefix: &cidr,
			Protocol:     6,
			PortLow:      443,
			PortHigh:     443,
			Action:       "allow",
		}},
	}

	if err := database.UpdatePolicyWithRules(ctx, created, rules); err != nil {
		t.Fatalf("UpdatePolicyWithRules: %v", err)
	}

	// Verify rules are visible before restart.
	beforeRules, err := database.ListRulesForPolicy(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy (before restart): %v", err)
	}

	if len(beforeRules) != 1 {
		t.Fatalf("expected 1 rule before restart, got %d", len(beforeRules))
	}

	// Close and reopen with Raft.
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	database2, err := db.NewDatabase(ctx, dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("NewDatabase (reopen): %v", err)
	}

	defer func() {
		if err := database2.Close(); err != nil {
			t.Fatalf("Close (reopen): %v", err)
		}
	}()

	afterPolicy, err := database2.GetPolicy(ctx, "raft-restart-policy")
	if err != nil {
		t.Fatalf("GetPolicy after restart: %v", err)
	}

	afterRules, err := database2.ListRulesForPolicy(ctx, afterPolicy.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy (after restart): %v", err)
	}

	if len(afterRules) != 1 {
		t.Fatalf("expected 1 rule after restart, got %d", len(afterRules))
	}

	if afterRules[0].Description != "allow-ipv6" {
		t.Fatalf("rule description mismatch: got %q", afterRules[0].Description)
	}
}

// TestUpdatePolicyWithRules_SnapshotRestore_SurviveRestart exercises the
// snapshot-restore path that triggered the original persistence bug.
//
// Scenario:
//  1. Create a policy (no rules) and force a Raft snapshot — the snapshot
//     captures fsm_state.lastApplied at the current log index.
//  2. Update the policy with rules — this changeset goes into the Raft log
//     AFTER the snapshot.
//  3. Close and reopen the database — Raft restores from the snapshot, then
//     replays post-snapshot log entries (including the rules changeset).
//
// Before the fix, fsm_state was in localOnlyTables. During restore, the
// pre-restore database's lastApplied (higher than the snapshot's) was
// preserved. The FSM then skipped replaying the post-snapshot changeset
// because l.Index <= lastApplied, silently losing the rules.
func TestUpdatePolicyWithRules_SnapshotRestore_SurviveRestart(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "data")

	database, err := db.NewDatabase(ctx, dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	// Create prerequisite entities.
	testDN := &db.DataNetwork{Name: "snap-dn", IPv4Pool: "10.88.0.0/24"}
	if err := database.CreateDataNetwork(ctx, testDN); err != nil {
		t.Fatalf("CreateDataNetwork: %v", err)
	}

	dn, err := database.GetDataNetwork(ctx, testDN.Name)
	if err != nil {
		t.Fatalf("GetDataNetwork: %v", err)
	}

	profileID, sliceID := createPolicyDeps(t, database, "snap-restore")

	policy := &db.Policy{
		Name:                "snap-restore-policy",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "100 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       dn.ID,
		ProfileID:           profileID,
		SliceID:             sliceID,
	}

	if err := database.CreatePolicy(ctx, policy); err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}

	created, err := database.GetPolicy(ctx, policy.Name)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}

	// Force a snapshot BEFORE adding rules. The snapshot captures the
	// current DB state (policy exists, no rules) and the corresponding
	// fsm_state.lastApplied.
	if err := database.ForceSnapshot(); err != nil {
		t.Fatalf("ForceSnapshot: %v", err)
	}

	// Now add rules — this changeset goes into the Raft log AFTER the
	// snapshot. On restart, Raft must replay it.
	cidr := "2001:db8:abcd::/48"
	rules := &db.PolicyRulesInput{
		Uplink: []db.PolicyRuleInput{{
			Description:  "post-snapshot-rule",
			RemotePrefix: &cidr,
			Protocol:     6,
			PortLow:      443,
			PortHigh:     443,
			Action:       "allow",
		}},
	}

	if err := database.UpdatePolicyWithRules(ctx, created, rules); err != nil {
		t.Fatalf("UpdatePolicyWithRules: %v", err)
	}

	// Verify rules exist before restart.
	beforeRules, err := database.ListRulesForPolicy(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy (before restart): %v", err)
	}

	if len(beforeRules) != 1 {
		t.Fatalf("expected 1 rule before restart, got %d", len(beforeRules))
	}

	// Close and reopen. On reopen, Raft restores from the snapshot (no
	// rules) and must replay the post-snapshot changeset (with rules).
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	database2, err := db.NewDatabase(ctx, dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("NewDatabase (reopen): %v", err)
	}

	defer func() {
		if err := database2.Close(); err != nil {
			t.Fatalf("Close (reopen): %v", err)
		}
	}()

	afterPolicy, err := database2.GetPolicy(ctx, "snap-restore-policy")
	if err != nil {
		t.Fatalf("GetPolicy after restart: %v", err)
	}

	afterRules, err := database2.ListRulesForPolicy(ctx, afterPolicy.ID)
	if err != nil {
		t.Fatalf("ListRulesForPolicy (after restart): %v", err)
	}

	if len(afterRules) != 1 {
		t.Fatalf("expected 1 rule after snapshot-restore restart, got %d", len(afterRules))
	}

	if afterRules[0].Description != "post-snapshot-rule" {
		t.Fatalf("rule description mismatch: got %q, want %q", afterRules[0].Description, "post-snapshot-rule")
	}
}
