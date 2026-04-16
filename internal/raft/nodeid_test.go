// Copyright 2026 Ella Networks

package raft

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveNodeID_FromConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	id, err := ResolveNodeID(5, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != 5 {
		t.Fatalf("want 5, got %d", id)
	}

	// Should have persisted to file.
	persisted := readPersistedID(t, dir)
	if persisted != 5 {
		t.Fatalf("persisted id: want 5, got %d", persisted)
	}
}

func TestResolveNodeID_FromEnv(t *testing.T) {
	t.Setenv(nodeIDEnvVar, "12")

	dir := t.TempDir()

	id, err := ResolveNodeID(0, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != 12 {
		t.Fatalf("want 12, got %d", id)
	}
}

func TestResolveNodeID_FromFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte("7\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	id, err := ResolveNodeID(0, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != 7 {
		t.Fatalf("want 7, got %d", id)
	}
}

func TestResolveNodeID_ConfigTakesPrecedenceOverEnv(t *testing.T) {
	t.Setenv(nodeIDEnvVar, "20")

	dir := t.TempDir()

	id, err := ResolveNodeID(10, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != 10 {
		t.Fatalf("config should take precedence: want 10, got %d", id)
	}
}

func TestResolveNodeID_EnvTakesPrecedenceOverFile(t *testing.T) {
	t.Setenv(nodeIDEnvVar, "20")

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte("30\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Env (20) differs from file (30) — this should fail with a mismatch error.
	_, err := ResolveNodeID(0, dir)
	if err == nil {
		t.Fatal("expected mismatch error")
	}

	if !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected mismatch error, got: %v", err)
	}
}

func TestResolveNodeID_MismatchConfigVsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte("3\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := ResolveNodeID(9, dir)
	if err == nil {
		t.Fatal("expected mismatch error when config differs from persisted file")
	}

	if !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected mismatch error, got: %v", err)
	}
}

func TestResolveNodeID_ConsistentConfigAndFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte("5\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	id, err := ResolveNodeID(5, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != 5 {
		t.Fatalf("want 5, got %d", id)
	}
}

func TestResolveNodeID_NoSourceAvailable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := ResolveNodeID(0, dir)
	if err == nil {
		t.Fatal("expected error when no source is available")
	}

	if !strings.Contains(err.Error(), "node ID not provided") {
		t.Fatalf("expected 'not provided' error, got: %v", err)
	}
}

func TestResolveNodeID_BoundaryMin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	id, err := ResolveNodeID(1, dir)
	if err != nil {
		t.Fatalf("unexpected error for ID 1: %v", err)
	}

	if id != 1 {
		t.Fatalf("want 1, got %d", id)
	}
}

func TestResolveNodeID_BoundaryMax(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	id, err := ResolveNodeID(MaxNodeID, dir)
	if err != nil {
		t.Fatalf("unexpected error for ID %d: %v", MaxNodeID, err)
	}

	if id != MaxNodeID {
		t.Fatalf("want %d, got %d", MaxNodeID, id)
	}
}

func TestResolveNodeID_BelowMin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := ResolveNodeID(-1, dir)
	if err == nil {
		t.Fatal("expected error for negative ID")
	}
}

func TestResolveNodeID_AboveMax(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := ResolveNodeID(MaxNodeID+1, dir)
	if err == nil {
		t.Fatalf("expected error for ID %d (above max)", MaxNodeID+1)
	}
}

func TestResolveNodeID_InvalidEnv(t *testing.T) {
	t.Setenv(nodeIDEnvVar, "not-a-number")

	dir := t.TempDir()

	_, err := ResolveNodeID(0, dir)
	if err == nil {
		t.Fatal("expected error for non-numeric env var")
	}

	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected 'invalid' error, got: %v", err)
	}
}

func TestResolveNodeID_CorruptFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte("garbage"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := ResolveNodeID(0, dir)
	if err == nil {
		t.Fatal("expected error for corrupt node-id file")
	}
}

func TestResolveNodeID_FileOutOfRange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, nodeIDFilename), []byte("999\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := ResolveNodeID(0, dir)
	if err == nil {
		t.Fatal("expected error for out-of-range persisted ID")
	}
}

func TestResolveNodeID_PersistOnFirstBoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// No file exists yet.
	_, err := os.Stat(filepath.Join(dir, nodeIDFilename))
	if err == nil {
		t.Fatal("node-id file should not exist yet")
	}

	id, err := ResolveNodeID(42, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != 42 {
		t.Fatalf("want 42, got %d", id)
	}

	// File should now exist.
	persisted := readPersistedID(t, dir)
	if persisted != 42 {
		t.Fatalf("persisted id: want 42, got %d", persisted)
	}

	// Second call with same config should succeed.
	id, err = ResolveNodeID(42, dir)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if id != 42 {
		t.Fatalf("second call: want 42, got %d", id)
	}
}

func readPersistedID(t testing.TB, dir string) int {
	t.Helper()

	id, err := readNodeIDFile(filepath.Join(dir, nodeIDFilename))
	if err != nil {
		t.Fatalf("readNodeIDFile: %v", err)
	}

	return id
}
