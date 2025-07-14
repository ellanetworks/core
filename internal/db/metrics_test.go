// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDatabaseMetrics(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(dbPath, initialOperator)
	if err != nil {
		t.Fatalf("Couldn't initialize NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't close database: %s", err)
		}
	}()

	profiles := []db.Profile{
		{Name: "Profile1", UeIPPool: "192.168.1.0/24"},
		{Name: "Profile2", UeIPPool: "10.0.0.0/16"},
	}
	for _, profile := range profiles {
		err := database.CreateProfile(context.Background(), &profile)
		if err != nil {
			t.Fatalf("Couldn't create profile: %s", err)
		}
	}

	subscribers := []db.Subscriber{
		{Imsi: "001", IPAddress: "192.168.1.2", ProfileID: 1},
		{Imsi: "002", IPAddress: "10.0.0.3", ProfileID: 2},
		{Imsi: "003", IPAddress: "", ProfileID: 1},
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

		expectedTotal := countIPsInCIDR("192.168.1.0/24") + countIPsInCIDR("10.0.0.0/16")
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
