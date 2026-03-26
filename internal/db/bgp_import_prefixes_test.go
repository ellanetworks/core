package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestImportPrefixes_EmptyByDefault(t *testing.T) {
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

	peer := &db.BGPPeer{
		Address:  "10.0.0.1",
		RemoteAS: 65001,
		HoldTime: 90,
	}

	err = database.CreateBGPPeer(context.Background(), peer)
	if err != nil {
		t.Fatalf("Couldn't create peer: %s", err)
	}

	prefixes, err := database.ListImportPrefixesByPeer(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("Couldn't list import prefixes: %s", err)
	}

	if len(prefixes) != 0 {
		t.Fatalf("Expected 0 prefixes by default, got %d", len(prefixes))
	}
}

func TestImportPrefixes_SetAndList(t *testing.T) {
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

	peer := &db.BGPPeer{
		Address:  "10.0.0.1",
		RemoteAS: 65001,
		HoldTime: 90,
	}

	err = database.CreateBGPPeer(context.Background(), peer)
	if err != nil {
		t.Fatalf("Couldn't create peer: %s", err)
	}

	entries := []db.BGPImportPrefix{
		{Prefix: "0.0.0.0/0", MaxLength: 0},
		{Prefix: "10.100.0.0/16", MaxLength: 24},
	}

	err = database.SetImportPrefixesForPeer(context.Background(), peer.ID, entries)
	if err != nil {
		t.Fatalf("Couldn't set import prefixes: %s", err)
	}

	prefixes, err := database.ListImportPrefixesByPeer(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("Couldn't list import prefixes: %s", err)
	}

	if len(prefixes) != 2 {
		t.Fatalf("Expected 2 prefixes, got %d", len(prefixes))
	}

	if prefixes[0].Prefix != "0.0.0.0/0" {
		t.Fatalf("Expected prefix 0.0.0.0/0, got %s", prefixes[0].Prefix)
	}

	if prefixes[0].MaxLength != 0 {
		t.Fatalf("Expected maxLength 0, got %d", prefixes[0].MaxLength)
	}

	if prefixes[1].Prefix != "10.100.0.0/16" {
		t.Fatalf("Expected prefix 10.100.0.0/16, got %s", prefixes[1].Prefix)
	}

	if prefixes[1].MaxLength != 24 {
		t.Fatalf("Expected maxLength 24, got %d", prefixes[1].MaxLength)
	}

	if prefixes[0].PeerID != peer.ID {
		t.Fatalf("Expected peerID %d, got %d", peer.ID, prefixes[0].PeerID)
	}
}

func TestImportPrefixes_ReplaceOnSet(t *testing.T) {
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

	peer := &db.BGPPeer{
		Address:  "10.0.0.1",
		RemoteAS: 65001,
		HoldTime: 90,
	}

	err = database.CreateBGPPeer(context.Background(), peer)
	if err != nil {
		t.Fatalf("Couldn't create peer: %s", err)
	}

	// Set initial prefixes
	err = database.SetImportPrefixesForPeer(context.Background(), peer.ID, []db.BGPImportPrefix{
		{Prefix: "0.0.0.0/0", MaxLength: 0},
		{Prefix: "10.0.0.0/8", MaxLength: 16},
	})
	if err != nil {
		t.Fatalf("Couldn't set initial import prefixes: %s", err)
	}

	// Replace with different prefixes
	err = database.SetImportPrefixesForPeer(context.Background(), peer.ID, []db.BGPImportPrefix{
		{Prefix: "172.16.0.0/12", MaxLength: 24},
	})
	if err != nil {
		t.Fatalf("Couldn't replace import prefixes: %s", err)
	}

	prefixes, err := database.ListImportPrefixesByPeer(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("Couldn't list import prefixes: %s", err)
	}

	if len(prefixes) != 1 {
		t.Fatalf("Expected 1 prefix after replace, got %d", len(prefixes))
	}

	if prefixes[0].Prefix != "172.16.0.0/12" {
		t.Fatalf("Expected prefix 172.16.0.0/12, got %s", prefixes[0].Prefix)
	}
}

func TestImportPrefixes_ClearWithEmptySlice(t *testing.T) {
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

	peer := &db.BGPPeer{
		Address:  "10.0.0.1",
		RemoteAS: 65001,
		HoldTime: 90,
	}

	err = database.CreateBGPPeer(context.Background(), peer)
	if err != nil {
		t.Fatalf("Couldn't create peer: %s", err)
	}

	err = database.SetImportPrefixesForPeer(context.Background(), peer.ID, []db.BGPImportPrefix{
		{Prefix: "0.0.0.0/0", MaxLength: 32},
	})
	if err != nil {
		t.Fatalf("Couldn't set import prefixes: %s", err)
	}

	// Clear by setting empty slice
	err = database.SetImportPrefixesForPeer(context.Background(), peer.ID, nil)
	if err != nil {
		t.Fatalf("Couldn't clear import prefixes: %s", err)
	}

	prefixes, err := database.ListImportPrefixesByPeer(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("Couldn't list import prefixes: %s", err)
	}

	if len(prefixes) != 0 {
		t.Fatalf("Expected 0 prefixes after clear, got %d", len(prefixes))
	}
}

func TestImportPrefixes_CascadeDeleteOnPeerRemoval(t *testing.T) {
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

	peer := &db.BGPPeer{
		Address:  "10.0.0.1",
		RemoteAS: 65001,
		HoldTime: 90,
	}

	err = database.CreateBGPPeer(context.Background(), peer)
	if err != nil {
		t.Fatalf("Couldn't create peer: %s", err)
	}

	err = database.SetImportPrefixesForPeer(context.Background(), peer.ID, []db.BGPImportPrefix{
		{Prefix: "0.0.0.0/0", MaxLength: 0},
	})
	if err != nil {
		t.Fatalf("Couldn't set import prefixes: %s", err)
	}

	// Delete the peer — prefixes should cascade-delete
	err = database.DeleteBGPPeer(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("Couldn't delete peer: %s", err)
	}

	prefixes, err := database.ListImportPrefixesByPeer(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("Couldn't list import prefixes: %s", err)
	}

	if len(prefixes) != 0 {
		t.Fatalf("Expected 0 prefixes after peer deletion (cascade), got %d", len(prefixes))
	}
}

func TestImportPrefixes_IsolatedBetweenPeers(t *testing.T) {
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

	peer1 := &db.BGPPeer{Address: "10.0.0.1", RemoteAS: 65001, HoldTime: 90}
	peer2 := &db.BGPPeer{Address: "10.0.0.2", RemoteAS: 65002, HoldTime: 90}

	err = database.CreateBGPPeer(context.Background(), peer1)
	if err != nil {
		t.Fatalf("Couldn't create peer1: %s", err)
	}

	err = database.CreateBGPPeer(context.Background(), peer2)
	if err != nil {
		t.Fatalf("Couldn't create peer2: %s", err)
	}

	err = database.SetImportPrefixesForPeer(context.Background(), peer1.ID, []db.BGPImportPrefix{
		{Prefix: "0.0.0.0/0", MaxLength: 0},
	})
	if err != nil {
		t.Fatalf("Couldn't set peer1 prefixes: %s", err)
	}

	err = database.SetImportPrefixesForPeer(context.Background(), peer2.ID, []db.BGPImportPrefix{
		{Prefix: "10.0.0.0/8", MaxLength: 24},
		{Prefix: "172.16.0.0/12", MaxLength: 16},
	})
	if err != nil {
		t.Fatalf("Couldn't set peer2 prefixes: %s", err)
	}

	p1Prefixes, err := database.ListImportPrefixesByPeer(context.Background(), peer1.ID)
	if err != nil {
		t.Fatalf("Couldn't list peer1 prefixes: %s", err)
	}

	if len(p1Prefixes) != 1 {
		t.Fatalf("Expected 1 prefix for peer1, got %d", len(p1Prefixes))
	}

	p2Prefixes, err := database.ListImportPrefixesByPeer(context.Background(), peer2.ID)
	if err != nil {
		t.Fatalf("Couldn't list peer2 prefixes: %s", err)
	}

	if len(p2Prefixes) != 2 {
		t.Fatalf("Expected 2 prefixes for peer2, got %d", len(p2Prefixes))
	}
}
