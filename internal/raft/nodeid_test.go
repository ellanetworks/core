// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestResolveRaftID_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()

	id, err := ResolveRaftID(dir)
	if err != nil {
		t.Fatalf("ResolveRaftID: %v", err)
	}

	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("a fresh node must take a UUID identity, got %q", id)
	}

	again, err := ResolveRaftID(dir)
	if err != nil {
		t.Fatalf("second ResolveRaftID: %v", err)
	}

	if again != id {
		t.Fatalf("identity changed across calls: %q then %q", id, again)
	}

	raw, err := os.ReadFile(filepath.Join(dir, nodeIDFilename))
	if err != nil {
		t.Fatalf("read node-id: %v", err)
	}

	if string(raw) != id+"\n" {
		t.Fatalf("node-id file holds %q, want %q", string(raw), id+"\n")
	}
}

func TestResolveRaftID_KeepsLegacyIntegerIdentity(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte("2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	id, err := ResolveRaftID(dir)
	if err != nil {
		t.Fatalf("ResolveRaftID: %v", err)
	}

	if id != "2" {
		t.Fatalf("a cluster formed before the identity split keeps its integer: got %q, want 2", id)
	}
}

func TestResolveRaftID_NormalizesCase(t *testing.T) {
	dir := t.TempDir()
	id := "0199C0DE-0000-7000-8000-00000000BEEF"

	if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte("  "+id+"  \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveRaftID(dir)
	if err != nil {
		t.Fatalf("ResolveRaftID: %v", err)
	}

	if got != "0199c0de-0000-7000-8000-00000000beef" {
		t.Fatalf("identity must be trimmed and lowercased once, got %q", got)
	}
}

func TestResolveRaftID_RejectsUnparseableFile(t *testing.T) {
	dir := t.TempDir()

	for _, content := range []string{"", "0", "64", "not-a-uuid", "1 2"} {
		if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		if _, err := ResolveRaftID(dir); err == nil {
			t.Fatalf("content %q must fail startup rather than mint a new identity", content)
		}
	}
}
