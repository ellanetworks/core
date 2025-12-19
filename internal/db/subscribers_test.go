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

func createDataNetworkAndPolicy(database *db.Database) (int, int, error) {
	newDataNetwork := &db.DataNetwork{
		Name:   "not-internet",
		IPPool: "1.2.3.0/24",
	}
	err := database.CreateDataNetwork(context.Background(), newDataNetwork)
	if err != nil {
		return 0, 0, err
	}

	createdNetwork, err := database.GetDataNetwork(context.Background(), newDataNetwork.Name)
	if err != nil {
		return 0, 0, err
	}

	policy := &db.Policy{
		Name:            "my-policy",
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		Arp:             1,
		DataNetworkID:   createdNetwork.ID,
	}

	err = database.CreatePolicy(context.Background(), policy)
	if err != nil {
		return 0, 0, err
	}

	policyCreated, err := database.GetPolicy(context.Background(), policy.Name)
	if err != nil {
		return 0, 0, err
	}

	return policyCreated.ID, createdNetwork.ID, nil
}

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

	res, total, err := database.ListSubscribersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("One or more subscribers were found in DB")
	}

	policyID, dataNetworkID, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and policy: %s", err)
	}

	subscriber := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "123456",
		PermanentKey:   "123456",
		Opc:            "123456",
		PolicyID:       policyID,
	}
	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, total, err = database.ListSubscribersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
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

	newPolicy := db.Policy{
		Name:          "another-policy",
		DataNetworkID: dataNetworkID,
	}
	err = database.CreatePolicy(context.Background(), &newPolicy)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	newPolicyCreated, err := database.GetPolicy(context.Background(), newPolicy.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	subscriber.PolicyID = newPolicyCreated.ID
	if err = database.UpdateSubscriberPolicy(context.Background(), subscriber); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedSubscriber, err = database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedSubscriber.PolicyID != newPolicyCreated.ID {
		t.Fatalf("Policy IDs don't match: %d", retrievedSubscriber.PolicyID)
	}

	if err = database.DeleteSubscriber(context.Background(), subscriber.Imsi); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}

	res, total, _ = database.ListSubscribersPage(context.Background(), 1, 10)

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

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
	if *retrievedSubscriber.IPAddress != allocatedIP.String() {
		t.Fatalf("IP address in database %s does not match allocated IP %s", *retrievedSubscriber.IPAddress, allocatedIP.String())
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
	if retrievedSubscriber.IPAddress != nil {
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

func TestCountSubscribersWithIP(t *testing.T) {
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

	count, err := database.CountSubscribersWithIP(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersWithIP: %s", err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 subscribers with IP, but got %d", count)
	}

	policyID, _, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and policy: %s", err)
	}

	ip := "192.168.1.2"
	subscriber1 := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "123456",
		PermanentKey:   "123456",
		Opc:            "123456",
		IPAddress:      &ip,
		PolicyID:       policyID,
	}
	err = database.CreateSubscriber(context.Background(), subscriber1)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	subscriber2 := &db.Subscriber{
		Imsi:           "001010100007488",
		SequenceNumber: "123457",
		PermanentKey:   "123457",
		Opc:            "123457",
		PolicyID:       policyID,
	}
	err = database.CreateSubscriber(context.Background(), subscriber2)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	count, err = database.CountSubscribersWithIP(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersWithIP: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 subscriber with IP, but got %d", count)
	}

	ip = "192.168.1.3"
	subscriber3 := &db.Subscriber{
		Imsi:           "001010100007489",
		SequenceNumber: "123458",
		PermanentKey:   "123458",
		Opc:            "123458",
		IPAddress:      &ip,
		PolicyID:       policyID,
	}
	err = database.CreateSubscriber(context.Background(), subscriber3)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	count, err = database.CountSubscribersWithIP(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersWithIP: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected 2 subscribers with IP, but got %d", count)
	}
}

func TestCountSubscribersInPolicy(t *testing.T) {
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

	policyID, dnID, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and policy: %s", err)
	}

	count, err := database.CountSubscribersInPolicy(context.Background(), policyID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInPolicy: %s", err)
	}

	if count != 0 {
		t.Fatalf("Expected 0 subscribers in policy, but got %d", count)
	}

	subscriber1 := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "123456",
		PermanentKey:   "123456",
		Opc:            "123456",
		PolicyID:       policyID,
	}
	err = database.CreateSubscriber(context.Background(), subscriber1)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	newPolicy := &db.Policy{
		Name:          "another-policy",
		DataNetworkID: dnID,
	}
	err = database.CreatePolicy(context.Background(), newPolicy)
	if err != nil {
		t.Fatalf("Couldn't Create Policy: %s", err)
	}

	newPolicyCreated, err := database.GetPolicy(context.Background(), newPolicy.Name)
	if err != nil {
		t.Fatalf("Couldn't Retrieve Policy: %s", err)
	}

	subscriber2 := &db.Subscriber{
		Imsi:           "001010100007488",
		SequenceNumber: "123457",
		PermanentKey:   "123457",
		Opc:            "123457",
		PolicyID:       newPolicyCreated.ID,
	}
	err = database.CreateSubscriber(context.Background(), subscriber2)
	if err != nil {
		t.Fatalf("Couldn't Create Subscriber: %s", err)
	}

	count, err = database.CountSubscribersInPolicy(context.Background(), policyID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInPolicy: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 subscriber in policy, but got %d", count)
	}

	subscriber3 := &db.Subscriber{
		Imsi:           "001010100007489",
		SequenceNumber: "123458",
		PermanentKey:   "123458",
		Opc:            "123458",
		PolicyID:       policyID,
	}
	err = database.CreateSubscriber(context.Background(), subscriber3)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	count, err = database.CountSubscribersInPolicy(context.Background(), policyID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInPolicy: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected 2 subscribers in policy, but got %d", count)
	}
}
