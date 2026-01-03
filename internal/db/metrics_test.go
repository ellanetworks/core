// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

const (
	DefaultDNIPPool = "10.45.0.0/22"
)

func TestDatabaseMetrics(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Couldn't initialize NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't close database: %s", err)
		}
	}()

	dataNetworks := []db.DataNetwork{
		{Name: "not-internet", IPPool: "10.0.0.0/16"},
	}
	for _, dn := range dataNetworks {
		err := database.CreateDataNetwork(context.Background(), &dn)
		if err != nil {
			t.Fatalf("Couldn't create data network: %s", err)
		}
	}

	policies := []db.Policy{
		{Name: "Policy1", DataNetworkID: 1},
		{Name: "Policy2", DataNetworkID: 1},
	}
	for _, policy := range policies {
		err := database.CreatePolicy(context.Background(), &policy)
		if err != nil {
			t.Fatalf("Couldn't create policy: %s", err)
		}
	}

	ip1 := "10.45.0.2"
	ip2 := "10.0.0.3"

	subscribers := []db.Subscriber{
		{
			Imsi:           "001019379926281",
			IPAddress:      &ip1,
			SequenceNumber: "000000000001",
			PolicyID:       1,
			Opc:            "1234567890abcdef1234567890abcdef",
			PermanentKey:   "1234567890abcdef1234567890abcdef",
		},
		{
			Imsi:           "001019379926282",
			IPAddress:      &ip2,
			SequenceNumber: "000000000002",
			PolicyID:       2,
			Opc:            "1234567890abcdef1234567890abcdef",
			PermanentKey:   "1234567890abcdef1234567890abcdef",
		},
		{
			Imsi:           "001019379926283",
			IPAddress:      nil,
			SequenceNumber: "000000000003",
			PolicyID:       1,
			Opc:            "1234567890abcdef1234567890abcdef",
			PermanentKey:   "1234567890abcdef1234567890abcdef",
		},
	}
	for _, subscriber := range subscribers {
		err := database.CreateSubscriber(context.Background(), &subscriber)
		if err != nil {
			t.Fatalf("Couldn't create subscriber: %s", err)
		}
	}

	t.Run("GetSize", func(t *testing.T) {
		size, err := database.GetSize()
		if err != nil {
			t.Fatalf("Couldn't get database size: %s", err)
		}

		if size == 0 {
			t.Fatalf("Database size should not be zero")
		}

		t.Logf("Database size: %d bytes", size)
	})

	t.Run("GetIPAddressesTotal", func(t *testing.T) {
		totalIPs, err := database.GetIPAddressesTotal()
		if err != nil {
			t.Fatalf("Couldn't get total IP addresses: %s", err)
		}

		expectedTotal := countIPsInCIDR(DefaultDNIPPool) + countIPsInCIDR("10.0.0.0/16")
		if totalIPs != expectedTotal {
			t.Fatalf("Expected total IPs %d, got %d", expectedTotal, totalIPs)
		}
	})

	t.Run("GetIPAddressesAllocated", func(t *testing.T) {
		allocatedIPs, err := database.GetIPAddressesAllocated(context.Background())
		if err != nil {
			t.Fatalf("Couldn't get allocated IP addresses: %s", err)
		}

		expectedAllocated := 2 // Two subscribers have allocated IPs
		if allocatedIPs != expectedAllocated {
			t.Fatalf("Expected allocated IPs %d, got %d", expectedAllocated, allocatedIPs)
		}
	})
}

// Helper function for IP range calculations
func countIPsInCIDR(cidr string) int {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}

	ones, bits := ipNet.Mask.Size()

	return 1 << (bits - ones)
}
