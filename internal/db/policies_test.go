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
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"), initialOperator)
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, err := database.ListPolicies(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more policies were found in DB")
	}

	dataNetwork := &db.DataNetwork{
		Name:   "internet",
		IPPool: "1.2.3.0/24",
	}
	err = database.CreateDataNetwork(context.Background(), dataNetwork)
	if err != nil {
		t.Fatalf("Couldn't complete CreateDataNetwork: %s", err)
	}

	createdNetwork, err := database.GetDataNetwork(context.Background(), dataNetwork.Name)
	if err != nil {
		t.Fatalf("Couldn't complete GetDataNetwork: %s", err)
	}

	policy := &db.Policy{
		Name:            "my-policy",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		PriorityLevel:   1,
		DataNetworkID:   createdNetwork.ID,
	}
	err = database.CreatePolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, err = database.ListPolicies(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 1 {
		t.Fatalf("One or more policies weren't found in DB")
	}

	retrievedPolicy, err := database.GetPolicy(context.Background(), policy.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedPolicy.Name != policy.Name {
		t.Fatalf("The policy name from the database doesn't match the policy name that was given")
	}
	if retrievedPolicy.BitrateUplink != policy.BitrateUplink {
		t.Fatalf("The bitrate uplink from the database doesn't match the bitrate uplink that was given")
	}
	if retrievedPolicy.BitrateDownlink != policy.BitrateDownlink {
		t.Fatalf("The bitrate downlink from the database doesn't match the bitrate downlink that was given")
	}
	if retrievedPolicy.Var5qi != policy.Var5qi {
		t.Fatalf("The Var5qi from the database doesn't match the Var5qi that was given")
	}
	if retrievedPolicy.PriorityLevel != policy.PriorityLevel {
		t.Fatalf("The priority level from the database doesn't match the priority level that was given")
	}

	// Edit the policy
	policy.Var5qi = 7
	policy.PriorityLevel = 2

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
	if retrievedPolicy.PriorityLevel != policy.PriorityLevel {
		t.Fatalf("The ue priority level from the database doesn't match the priority level that was given")
	}

	if err = database.DeletePolicy(context.Background(), policy.Name); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListPolicies(context.Background())
	if len(res) != 0 {
		t.Fatalf("Policies weren't deleted from the DB properly")
	}
}
