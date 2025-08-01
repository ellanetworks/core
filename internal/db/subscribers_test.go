// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestSubscribersDbEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, err := database.ListSubscribers(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more subscribers were found in DB")
	}

	subscriber := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "123456",
		PermanentKey:   "123456",
		Opc:            "123456",
	}
	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, err = database.ListSubscribers(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 1 {
		t.Fatalf("One or more subscribers weren't found in DB")
	}

	retrievedSubscriber, err := database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedSubscriber.Imsi != subscriber.Imsi {
		t.Fatalf("The subscriber from the database doesn't match the subscriber that was given")
	}
	if retrievedSubscriber.SequenceNumber != subscriber.SequenceNumber {
		t.Fatalf("The sequence number from the database doesn't match the sequence number that was given")
	}
	if retrievedSubscriber.PermanentKey != subscriber.PermanentKey {
		t.Fatalf("The permanent key value from the database doesn't match the permanent key value that was given")
	}
	if retrievedSubscriber.Opc != subscriber.Opc {
		t.Fatalf("The OPC value from the database doesn't match the OPC value that was given")
	}

	policyData := &db.Policy{
		Name: "mypolicyname",
	}
	err = database.CreatePolicy(context.Background(), policyData)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	subscriber.SequenceNumber = "654321"
	if err = database.UpdateSubscriber(context.Background(), subscriber); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedSubscriber, err = database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedSubscriber.SequenceNumber != "654321" {
		t.Fatalf("Sequence numbers don't match: %s", retrievedSubscriber.SequenceNumber)
	}

	if err = database.DeleteSubscriber(context.Background(), subscriber.Imsi); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListSubscribers(context.Background())
	if len(res) != 0 {
		t.Fatalf("Subscribers weren't deleted from the DB properly")
	}
}

func TestIPAllocationAndRelease(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dnn := &db.DataNetwork{
		Name:   "test-dnn",
		IPPool: "192.168.1.0/24",
	}

	err = database.CreateDataNetwork(context.Background(), dnn)
	if err != nil {
		t.Fatalf("Couldn't complete CreateDataNetwork: %s", err)
	}

	createdDNN, err := database.GetDataNetwork(context.Background(), dnn.Name)
	if err != nil {
		t.Fatalf("Couldn't retrieve data network: %s", err)
	}

	policy := &db.Policy{
		Name:          "test-policy",
		DataNetworkID: createdDNN.ID,
	}
	err = database.CreatePolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("Couldn't complete CreatePolicy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(context.Background(), policy.Name)
	if err != nil {
		t.Fatalf("Couldn't retrieve policy: %s", err)
	}

	subscriber := &db.Subscriber{
		Imsi:           "001010123456789",
		SequenceNumber: "123456",
		PermanentKey:   "abcdef",
		Opc:            "123456",
		PolicyID:       createdPolicy.ID,
	}
	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	// Step 3: Allocate an IP for the subscriber
	allocatedIP, err := database.AllocateIP(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't allocate IP for subscriber: %s", err)
	}
	if allocatedIP == nil {
		t.Fatalf("Allocated IP is nil")
	}

	// Verify that the allocated IP is within the policy's IP pool
	_, ipNet, _ := net.ParseCIDR(dnn.IPPool)
	if !ipNet.Contains(allocatedIP) {
		t.Fatalf("Allocated IP %s is not within the policy's IP pool %s", allocatedIP.String(), dnn.IPPool)
	}

	retrievedSubscriber, err := database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't retrieve subscriber: %s", err)
	}
	if retrievedSubscriber.IPAddress != allocatedIP.String() {
		t.Fatalf("IP address in database %s does not match allocated IP %s", retrievedSubscriber.IPAddress, allocatedIP.String())
	}

	// Step 5: Release the IP
	err = database.ReleaseIP(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't release IP for subscriber: %s", err)
	}

	// Verify that the IP is cleared in the database
	retrievedSubscriber, err = database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't retrieve subscriber after release: %s", err)
	}
	if retrievedSubscriber.IPAddress != "" {
		t.Fatalf("IP address was not cleared from the database after release")
	}

	// Step 6: Reallocate an IP for the same subscriber
	newAllocatedIP, err := database.AllocateIP(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't allocate a new IP for subscriber: %s", err)
	}
	if newAllocatedIP == nil {
		t.Fatalf("New allocated IP is nil")
	}
}

func TestAllocateAllIPsInPool(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	dnn := &db.DataNetwork{
		Name:   "test-dnn",
		IPPool: "192.168.1.0/29", // Small pool for testing (6 usable addresses)
	}
	err = database.CreateDataNetwork(context.Background(), dnn)
	if err != nil {
		t.Fatalf("Couldn't complete CreateDataNetwork: %s", err)
	}

	createdDNN, err := database.GetDataNetwork(context.Background(), dnn.Name)
	if err != nil {
		t.Fatalf("Couldn't retrieve data network: %s", err)
	}

	// Create a policy with an IP pool
	policy := &db.Policy{
		Name:          "test-pool",
		DataNetworkID: createdDNN.ID,
	}
	err = database.CreatePolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("Couldn't complete CreatePolicy: %s", err)
	}

	createdPolicy, err := database.GetPolicy(context.Background(), policy.Name)
	if err != nil {
		t.Fatalf("Couldn't retrieve policy: %s", err)
	}

	// Allocate all IPs in the pool
	allocatedIPs := make(map[string]struct{})
	_, ipNet, _ := net.ParseCIDR(dnn.IPPool)
	maskBits, totalBits := ipNet.Mask.Size()
	totalIPs := 1 << (totalBits - maskBits)

	for i := 1; i < totalIPs-1; i++ { // Skip network and broadcast addresses
		subscriber := &db.Subscriber{
			Imsi:           fmt.Sprintf("IMSI%012d", i),
			SequenceNumber: fmt.Sprintf("%d", i),
			PermanentKey:   fmt.Sprintf("%d", i),
			Opc:            fmt.Sprintf("%d", i),
			PolicyID:       createdPolicy.ID,
		}

		err := database.CreateSubscriber(context.Background(), subscriber)
		if err != nil {
			t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
		}

		allocatedIP, err := database.AllocateIP(context.Background(), subscriber.Imsi)
		if err != nil {
			t.Fatalf("Couldn't allocate IP for subscriber %s: %s", subscriber.Imsi, err)
		}
		if allocatedIP == nil {
			t.Fatalf("Allocated IP is nil for subscriber %s", subscriber.Imsi)
		}

		ipStr := allocatedIP.String()
		if _, exists := allocatedIPs[ipStr]; exists {
			t.Fatalf("Duplicate IP allocation detected: %s", ipStr)
		}
		allocatedIPs[ipStr] = struct{}{}

		// Verify that the allocated IP is within the pool
		if !ipNet.Contains(allocatedIP) {
			t.Fatalf("Allocated IP %s is not within the policy's IP pool %s", ipStr, dnn.IPPool)
		}
	}

	// Attempt to allocate one more IP, which should fail
	extraSubscriber := &db.Subscriber{
		Imsi:           "IMSI_OVERFLOW",
		SequenceNumber: "123456",
		PermanentKey:   "abcdef",
		Opc:            "123456",
		PolicyID:       createdPolicy.ID,
	}
	err = database.CreateSubscriber(context.Background(), extraSubscriber)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber for overflow subscriber: %s", err)
	}

	_, err = database.AllocateIP(context.Background(), extraSubscriber.Imsi)
	if err == nil {
		t.Fatalf("Expected error when allocating IP for subscriber %s, but no error occurred", extraSubscriber.Imsi)
	}

	if len(allocatedIPs) != totalIPs-2 { // Total IPs minus network and broadcast
		t.Fatalf("Expected %d allocated IPs, but got %d", totalIPs-2, len(allocatedIPs))
	}
}
