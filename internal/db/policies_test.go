// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestPoliciesEndToEnd(t *testing.T) {
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

	res, total, err := database.ListPoliciesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 1 {
		t.Fatalf("Default policy wasn't found in DB")
	}

	if len(res) != 1 {
		t.Fatalf("More than one policies were found in DB")
	}

	newDataNetwork := &db.DataNetwork{
		Name:   "not-internet",
		IPPool: "1.2.3.0/24",
	}

	err = database.CreateDataNetwork(context.Background(), newDataNetwork)
	if err != nil {
		t.Fatalf("Couldn't complete CreateDataNetwork: %s", err)
	}

	createdNetwork, err := database.GetDataNetwork(context.Background(), newDataNetwork.Name)
	if err != nil {
		t.Fatalf("Couldn't complete GetDataNetwork: %s", err)
	}

	policy := &db.Policy{
		Name:                "my-policy",
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
		DataNetworkID:       createdNetwork.ID,
		ProfileID:           1,
		SliceID:             1,
	}

	err = database.CreatePolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, total, err = database.ListPoliciesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 2 {
		t.Fatalf("Not all policies were found in DB")
	}

	if len(res) != 2 {
		t.Fatalf("One or more policies weren't found in DB")
	}

	retrievedPolicy, err := database.GetPolicy(context.Background(), policy.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedPolicy.Name != policy.Name {
		t.Fatalf("The policy name from the database doesn't match the policy name that was given")
	}

	if retrievedPolicy.SessionAmbrUplink != policy.SessionAmbrUplink {
		t.Fatalf("The bitrate uplink from the database doesn't match the bitrate uplink that was given")
	}

	if retrievedPolicy.SessionAmbrDownlink != policy.SessionAmbrDownlink {
		t.Fatalf("The bitrate downlink from the database doesn't match the bitrate downlink that was given")
	}

	if retrievedPolicy.Var5qi != policy.Var5qi {
		t.Fatalf("The Var5qi from the database doesn't match the Var5qi that was given")
	}

	if retrievedPolicy.Arp != policy.Arp {
		t.Fatalf("The ARP from the database doesn't match the ARP that was given")
	}

	// Edit the policy
	policy.Var5qi = 7
	policy.Arp = 2

	if err = database.UpdatePolicy(context.Background(), policy); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedPolicy, err = database.GetPolicy(context.Background(), policy.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedPolicy.Name != policy.Name {
		t.Fatalf("The policy name from the database doesn't match the policy name that was given")
	}

	if retrievedPolicy.Var5qi != policy.Var5qi {
		t.Fatalf("The 5qi from the database doesn't match the 5qi that was given")
	}

	if retrievedPolicy.Arp != policy.Arp {
		t.Fatalf("The ARP from the database doesn't match the ARP that was given")
	}

	if err = database.DeletePolicy(context.Background(), policy.Name); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}

	res, total, _ = database.ListPoliciesPage(context.Background(), 1, 10)
	if total != 1 {
		t.Fatalf("Policy wasn't deleted from the DB properly")
	}

	if len(res) != 1 {
		t.Fatalf("Policy wasn't deleted from the DB properly")
	}
}

func TestGetPolicyByLookup(t *testing.T) {
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

	// The default policy links default profile (1), default slice (1), default data network (1)
	policy, err := database.GetPolicyByLookup(context.Background(), 1, 1, 1)
	if err != nil {
		t.Fatalf("Couldn't complete GetPolicyByLookup: %s", err)
	}

	if policy.Name != "default" {
		t.Fatalf("Expected default policy, got %q", policy.Name)
	}

	// Non-existent lookup
	_, err = database.GetPolicyByLookup(context.Background(), 999, 999, 999)
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}
}

func TestGetPolicyByProfileAndSlice(t *testing.T) {
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

	// Default profile ID = 1, slice ID = 1 has the default policy
	policy, err := database.GetPolicyByProfileAndSlice(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("Couldn't complete GetPolicyByProfileAndSlice: %s", err)
	}

	if policy.Name != "default" {
		t.Fatalf("Expected default policy, got %q", policy.Name)
	}

	// Non-existent profile + slice combination
	_, err = database.GetPolicyByProfileAndSlice(context.Background(), 999, 999)
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}
}

func TestCountPoliciesInRelations(t *testing.T) {
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

	// Default policy references profile 1, slice 1, data network 1
	count, err := database.CountPoliciesInProfile(context.Background(), 1)
	if err != nil {
		t.Fatalf("Couldn't complete CountPoliciesInProfile: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 policy in default profile, got %d", count)
	}

	count, err = database.CountPoliciesInSlice(context.Background(), 1)
	if err != nil {
		t.Fatalf("Couldn't complete CountPoliciesInSlice: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 policy in default slice, got %d", count)
	}

	count, err = database.CountPoliciesInDataNetwork(context.Background(), 1)
	if err != nil {
		t.Fatalf("Couldn't complete CountPoliciesInDataNetwork: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 policy in default data network, got %d", count)
	}

	// Non-existent relations
	count, err = database.CountPoliciesInProfile(context.Background(), 999)
	if err != nil {
		t.Fatalf("Couldn't complete CountPoliciesInProfile: %s", err)
	}

	if count != 0 {
		t.Fatalf("Expected 0 policies for non-existent profile, got %d", count)
	}
}

func TestPoliciesInDataNetworkAndSlice(t *testing.T) {
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

	// Default data network "internet" has the default policy
	exists, err := database.PoliciesInDataNetwork(context.Background(), "internet")
	if err != nil {
		t.Fatalf("Couldn't complete PoliciesInDataNetwork: %s", err)
	}

	if !exists {
		t.Fatal("Expected policies to exist for 'internet' data network")
	}

	// Default slice "default" has the default policy
	exists, err = database.PoliciesInSlice(context.Background(), "default")
	if err != nil {
		t.Fatalf("Couldn't complete PoliciesInSlice: %s", err)
	}

	if !exists {
		t.Fatal("Expected policies to exist for 'default' slice")
	}

	// Create an unused data network
	err = database.CreateDataNetwork(context.Background(), &db.DataNetwork{
		Name:   "unused-dn",
		IPPool: "172.16.0.0/24",
	})
	if err != nil {
		t.Fatalf("Couldn't complete CreateDataNetwork: %s", err)
	}

	exists, err = database.PoliciesInDataNetwork(context.Background(), "unused-dn")
	if err != nil {
		t.Fatalf("Couldn't complete PoliciesInDataNetwork: %s", err)
	}

	if exists {
		t.Fatal("Expected no policies for unused data network")
	}
}

func TestGetSessionPolicy(t *testing.T) {
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

	// Create a subscriber on the default profile
	subscriber := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      1, // default profile
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	// Default slice: sst=1, sd=""; Default DNN: "internet"
	policy, rules, dn, err := database.GetSessionPolicy(context.Background(), "001010100007487", 1, "", "internet")
	if err != nil {
		t.Fatalf("Couldn't complete GetSessionPolicy: %s", err)
	}

	if policy.Name != "default" {
		t.Fatalf("Expected default policy, got %q", policy.Name)
	}

	if rules == nil {
		t.Fatal("Expected non-nil rules slice")
	}

	if dn == nil {
		t.Fatal("Expected non-nil data network")
	}

	// Non-existent subscriber
	_, _, _, err = database.GetSessionPolicy(context.Background(), "999999999999999", 1, "", "internet") //nolint:dogsled // error-path test
	if err == nil {
		t.Fatal("Expected error for non-existent subscriber")
	}

	// Non-matching slice
	_, _, _, err = database.GetSessionPolicy(context.Background(), "001010100007487", 99, "ffffff", "internet") //nolint:dogsled // error-path test
	if err == nil {
		t.Fatal("Expected error for non-matching slice")
	}

	// Non-matching DNN
	_, _, _, err = database.GetSessionPolicy(context.Background(), "001010100007487", 1, "", "nonexistent-dnn") //nolint:dogsled // error-path test
	if err == nil {
		t.Fatal("Expected error for non-matching DNN")
	}
}
