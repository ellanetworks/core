package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDBBGPPeersEndToEnd(t *testing.T) {
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

	// List should be empty initially
	peers, total, err := database.ListBGPPeersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't list BGP peers: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, got %d", total)
	}

	if len(peers) != 0 {
		t.Fatalf("Expected no peers, got %d", len(peers))
	}

	// Count should be 0
	count, err := database.CountBGPPeers(context.Background())
	if err != nil {
		t.Fatalf("Couldn't count BGP peers: %s", err)
	}

	if count != 0 {
		t.Fatalf("Expected count to be 0, got %d", count)
	}

	// Create a peer
	peer := &db.BGPPeer{
		Address:     "192.168.1.1",
		RemoteAS:    64512,
		HoldTime:    90,
		Description: "test-peer",
	}

	err = database.CreateBGPPeer(context.Background(), peer)
	if err != nil {
		t.Fatalf("Couldn't create BGP peer: %s", err)
	}

	// List should have 1 peer
	peers, total, err = database.ListBGPPeersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't list BGP peers: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, got %d", total)
	}

	if len(peers) != 1 {
		t.Fatalf("Expected 1 peer, got %d", len(peers))
	}

	peerID := peers[0].ID

	if peers[0].Address != "192.168.1.1" {
		t.Fatalf("Expected address 192.168.1.1, got %s", peers[0].Address)
	}

	if peers[0].RemoteAS != 64512 {
		t.Fatalf("Expected remoteAS 64512, got %d", peers[0].RemoteAS)
	}

	if peers[0].HoldTime != 90 {
		t.Fatalf("Expected holdTime 90, got %d", peers[0].HoldTime)
	}

	if peers[0].Description != "test-peer" {
		t.Fatalf("Expected description 'test-peer', got %s", peers[0].Description)
	}

	// Get peer by ID
	retrievedPeer, err := database.GetBGPPeer(context.Background(), peerID)
	if err != nil {
		t.Fatalf("Couldn't get BGP peer: %s", err)
	}

	if retrievedPeer.Address != "192.168.1.1" {
		t.Fatalf("Expected address 192.168.1.1, got %s", retrievedPeer.Address)
	}

	if retrievedPeer.RemoteAS != 64512 {
		t.Fatalf("Expected remoteAS 64512, got %d", retrievedPeer.RemoteAS)
	}

	// Count should be 1
	count, err = database.CountBGPPeers(context.Background())
	if err != nil {
		t.Fatalf("Couldn't count BGP peers: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected count to be 1, got %d", count)
	}

	// Delete peer
	err = database.DeleteBGPPeer(context.Background(), peerID)
	if err != nil {
		t.Fatalf("Couldn't delete BGP peer: %s", err)
	}

	// List should be empty again
	peers, total, err = database.ListBGPPeersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't list BGP peers: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, got %d", total)
	}

	if len(peers) != 0 {
		t.Fatalf("Expected no peers, got %d", len(peers))
	}
}

func TestCreateBGPPeer_DuplicateAddress(t *testing.T) {
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

	peer := &db.BGPPeer{
		Address:  "10.0.0.1",
		RemoteAS: 64512,
		HoldTime: 90,
	}

	err = database.CreateBGPPeer(context.Background(), peer)
	if err != nil {
		t.Fatalf("Couldn't create first BGP peer: %s", err)
	}

	// Creating a peer with the same address should fail
	duplicate := &db.BGPPeer{
		Address:  "10.0.0.1",
		RemoteAS: 64513,
		HoldTime: 90,
	}

	err = database.CreateBGPPeer(context.Background(), duplicate)
	if err != db.ErrAlreadyExists {
		t.Fatalf("Expected ErrAlreadyExists, got %v", err)
	}
}

func TestGetBGPPeer_NotFound(t *testing.T) {
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

	_, err = database.GetBGPPeer(context.Background(), 9999)
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}
}

func TestDeleteBGPPeer_NotFound(t *testing.T) {
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

	err = database.DeleteBGPPeer(context.Background(), 9999)
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}
}

func TestListAllBGPPeers(t *testing.T) {
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

	// Initially empty
	peers, err := database.ListAllBGPPeers(context.Background())
	if err != nil {
		t.Fatalf("Couldn't list all BGP peers: %s", err)
	}

	if len(peers) != 0 {
		t.Fatalf("Expected no peers, got %d", len(peers))
	}

	// Create two peers
	for _, addr := range []string{"10.0.0.1", "10.0.0.2"} {
		peer := &db.BGPPeer{
			Address:  addr,
			RemoteAS: 64512,
			HoldTime: 90,
		}

		err = database.CreateBGPPeer(context.Background(), peer)
		if err != nil {
			t.Fatalf("Couldn't create BGP peer %s: %s", addr, err)
		}
	}

	// Should return both
	peers, err = database.ListAllBGPPeers(context.Background())
	if err != nil {
		t.Fatalf("Couldn't list all BGP peers: %s", err)
	}

	if len(peers) != 2 {
		t.Fatalf("Expected 2 peers, got %d", len(peers))
	}
}

func TestCreateMultipleBGPPeers(t *testing.T) {
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

	addresses := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}

	for _, addr := range addresses {
		peer := &db.BGPPeer{
			Address:     addr,
			RemoteAS:    64512,
			HoldTime:    90,
			Description: "peer-" + addr,
		}

		err = database.CreateBGPPeer(context.Background(), peer)
		if err != nil {
			t.Fatalf("Couldn't create BGP peer %s: %s", addr, err)
		}
	}

	count, err := database.CountBGPPeers(context.Background())
	if err != nil {
		t.Fatalf("Couldn't count BGP peers: %s", err)
	}

	if count != 3 {
		t.Fatalf("Expected count to be 3, got %d", count)
	}

	// Test pagination
	peers, total, err := database.ListBGPPeersPage(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("Couldn't list BGP peers page: %s", err)
	}

	if total != 3 {
		t.Fatalf("Expected total 3, got %d", total)
	}

	if len(peers) != 2 {
		t.Fatalf("Expected 2 peers on page 1, got %d", len(peers))
	}

	peers, total, err = database.ListBGPPeersPage(context.Background(), 2, 2)
	if err != nil {
		t.Fatalf("Couldn't list BGP peers page 2: %s", err)
	}

	if total != 3 {
		t.Fatalf("Expected total 3, got %d", total)
	}

	if len(peers) != 1 {
		t.Fatalf("Expected 1 peer on page 2, got %d", len(peers))
	}
}
