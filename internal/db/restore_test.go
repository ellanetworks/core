// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestRestore(t *testing.T) {
	tempDir := t.TempDir()

	databasePath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(context.Background(), databasePath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
	}()

	// Create a real SQLite backup using the Backup method
	backupFile, err := os.CreateTemp("", "backup_*.db")
	if err != nil {
		t.Fatalf("failed to create temporary backup file: %v", err)
	}

	defer func() {
		err := backupFile.Close()
		if err != nil {
			t.Fatalf("failed to close backup file: %v", err)
		}

		err = os.Remove(backupFile.Name()) // Ensure cleanup
		if err != nil {
			t.Fatalf("failed to remove backup file: %v", err)
		}
	}()

	if err := database.Backup(context.Background(), backupFile); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if _, err := backupFile.Seek(0, 0); err != nil {
		t.Fatalf("failed to reset backup file pointer: %v", err)
	}

	err = database.Restore(context.Background(), backupFile)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify the restored database is functional by running a query
	_, total, err := database.ListSubscribersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("failed to query restored database: %v", err)
	}

	if total != 0 {
		t.Fatalf("expected 0 subscribers, got %d", total)
	}
}

func TestRestore_InvalidFile(t *testing.T) {
	tempDir := t.TempDir()
	databasePath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(context.Background(), databasePath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
	}()

	// Write garbage data to a temp file
	invalidFile, err := os.CreateTemp("", "invalid_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	defer func() {
		_ = invalidFile.Close()
		_ = os.Remove(invalidFile.Name())
	}()

	if _, err := invalidFile.WriteString("this is not a sqlite database"); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}

	if _, err := invalidFile.Seek(0, 0); err != nil {
		t.Fatalf("failed to seek: %v", err)
	}

	err = database.Restore(context.Background(), invalidFile)
	if err == nil {
		t.Fatal("expected error for invalid backup file, got nil")
	}

	if !errors.Is(err, db.ErrInvalidBackupFile) {
		t.Fatalf("expected ErrInvalidBackupFile, got: %v", err)
	}

	// Verify the original database is still functional
	_, total, err := database.ListSubscribersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("database should still be functional after rejected restore, got: %v", err)
	}

	if total != 0 {
		t.Fatalf("expected 0 subscribers, got %d", total)
	}
}

func TestRestore_ConcurrentRestore(t *testing.T) {
	tempDir := t.TempDir()
	databasePath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(context.Background(), databasePath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
	}()

	// Create two valid backup files
	makeBackup := func(name string) string {
		path := filepath.Join(tempDir, name)

		f, err := os.Create(path) //nolint:gosec // test-only; path is from t.TempDir()
		if err != nil {
			t.Fatalf("failed to create backup file %s: %v", name, err)
		}

		if err := database.Backup(context.Background(), f); err != nil {
			_ = f.Close()

			t.Fatalf("failed to create backup %s: %v", name, err)
		}

		_ = f.Close()

		return path
	}

	backupPath1 := makeBackup("backup1.db")
	backupPath2 := makeBackup("backup2.db")

	var wg sync.WaitGroup

	errs := make([]error, 2)

	wg.Add(2)

	restoreFromPath := func(idx int, path string) {
		defer wg.Done()

		f, err := os.Open(path) //nolint:gosec // test-only; path is from t.TempDir()
		if err != nil {
			errs[idx] = err
			return
		}

		defer func() { _ = f.Close() }()

		errs[idx] = database.Restore(context.Background(), f)
	}

	go restoreFromPath(0, backupPath1)
	go restoreFromPath(1, backupPath2)

	wg.Wait()

	// Exactly one should succeed and one should get ErrRestoreInProgress.
	var successCount, inProgressCount int

	for _, err := range errs {
		if err == nil {
			successCount++
		} else if errors.Is(err, db.ErrRestoreInProgress) {
			inProgressCount++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if successCount != 1 {
		t.Fatalf("expected exactly 1 success, got %d", successCount)
	}

	if inProgressCount != 1 {
		t.Fatalf("expected exactly 1 ErrRestoreInProgress, got %d", inProgressCount)
	}

	// Database should still be functional
	_, total, err := database.ListSubscribersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("database should be functional after concurrent restore, got: %v", err)
	}

	if total != 0 {
		t.Fatalf("expected 0 subscribers, got %d", total)
	}
}

// TestRestore_RoundTripPreservesData populates a database with rows in both
// replicated tables and local-only tables, takes a backup, mutates the live
// data, then restores the backup. Replicated rows come from the backup image;
// local-only rows are preserved from the current node state.
func TestRestore_RoundTripPreservesData(t *testing.T) {
	tempDir := t.TempDir()
	databasePath := filepath.Join(tempDir, "data")
	ctx := context.Background()

	database, err := db.NewDatabase(ctx, databasePath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
	}()

	const imsi = "001010000000042"

	if _, err := createDataNetworkPolicyAndSubscriber(database, imsi); err != nil {
		t.Fatalf("failed to seed subscriber: %v", err)
	}

	flow := &dbwriter.FlowReport{
		SubscriberID:    imsi,
		SourceIP:        "192.168.1.10",
		DestinationIP:   "8.8.8.8",
		SourcePort:      54321,
		DestinationPort: 443,
		Protocol:        6,
		Packets:         42,
		Bytes:           1337,
		StartTime:       time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		EndTime:         time.Now().UTC().Format(time.RFC3339),
		Direction:       "uplink",
	}

	if err := database.InsertFlowReports(ctx, []*dbwriter.FlowReport{flow}); err != nil {
		t.Fatalf("failed to insert flow report: %v", err)
	}

	// Take a backup of the populated database.
	backupFile, err := os.CreateTemp(tempDir, "backup_*.tar.gz")
	if err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}

	defer func() {
		_ = backupFile.Close()
		_ = os.Remove(backupFile.Name())
	}()

	if err := database.Backup(ctx, backupFile); err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	// Mutate the live data so we can prove the restore actually replaces it.
	if err := database.DeleteSubscriber(ctx, imsi); err != nil {
		t.Fatalf("delete subscriber failed: %v", err)
	}

	if err := database.ClearFlowReports(ctx); err != nil {
		t.Fatalf("clear flow reports failed: %v", err)
	}

	subsAfterDelete, totalSubs, err := database.ListSubscribersPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list after delete failed: %v", err)
	}

	if totalSubs != 0 || len(subsAfterDelete) != 0 {
		t.Fatalf("expected 0 subscribers after delete, got %d", totalSubs)
	}

	reportsAfterClear, totalReports, err := database.ListFlowReports(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("list flow reports after clear failed: %v", err)
	}

	if totalReports != 0 || len(reportsAfterClear) != 0 {
		t.Fatalf("expected 0 flow reports after clear, got %d", totalReports)
	}

	if _, err := backupFile.Seek(0, 0); err != nil {
		t.Fatalf("rewind backup failed: %v", err)
	}

	if err := database.Restore(ctx, backupFile); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Subscriber from shared.db should be back.
	subs, total, err := database.ListSubscribersPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list subscribers after restore failed: %v", err)
	}

	if total != 1 || len(subs) != 1 {
		t.Fatalf("expected 1 subscriber after restore, got total=%d len=%d", total, len(subs))
	}

	if subs[0].Imsi != imsi {
		t.Fatalf("expected imsi %q, got %q", imsi, subs[0].Imsi)
	}

	// Local-only flow reports are preserved from the current node state rather
	// than restored from the backup image, so the explicit clear above remains.
	reports, total, err := database.ListFlowReports(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("list flow reports after restore failed: %v", err)
	}

	if total != 0 || len(reports) != 0 {
		t.Fatalf("expected local-only flow reports to remain cleared after restore, got total=%d len=%d", total, len(reports))
	}

	// Only local.db gets a safety copy; shared.db is replicated through the
	// Raft log via CmdRestore. The local safety copy must be cleaned up
	// after a successful restore.
	if _, err := os.Stat(filepath.Join(database.Dir(), "restore_safety_local.db")); !os.IsNotExist(err) {
		t.Fatalf("expected restore_safety_local.db to be removed after successful restore, got err=%v", err)
	}
}
