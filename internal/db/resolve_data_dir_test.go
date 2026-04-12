// Copyright 2026 Ella Networks

package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// seedLegacyV8 creates a legacy single-file SQLite database at the given path,
// runs all legacyMigrations to bring it to v8, and inserts a couple of rows so
// the migration can verify row counts copy faithfully.
func seedLegacyV8(t *testing.T, path string) {
	t.Helper()

	ctx := context.Background()

	conn, err := openSQLiteConnection(ctx, path, SyncFull)
	if err != nil {
		t.Fatalf("failed to open seed legacy db: %v", err)
	}

	if migrErr := runLegacyMigrations(ctx, conn); migrErr != nil {
		_ = conn.Close()

		t.Fatalf("failed to run legacy migrations: %v", migrErr)
	}

	// Seed a couple of rows so the row-count check is non-trivial.
	exec := func(stmt string) {
		if _, execErr := conn.ExecContext(ctx, stmt); execErr != nil {
			_ = conn.Close()

			t.Fatalf("seed failed: %s\n%v", stmt, execErr)
		}
	}

	exec("INSERT INTO data_networks (id, name, ipPool, dns, mtu) VALUES (1, 'internet', '10.0.0.0/24', '8.8.8.8', 1500)")
	exec("INSERT INTO network_slices (id, sst, sd, name) VALUES (1, 1, NULL, 'default')")
	exec("INSERT INTO profiles (id, name, ueAmbrUplink, ueAmbrDownlink) VALUES (1, 'default', '200 Mbps', '200 Mbps')")
	exec("INSERT INTO policies (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink) " +
		"VALUES (1, 'default', 1, 1, 1, 9, 1, '200 Mbps', '200 Mbps')")
	exec("INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, profileID) " +
		"VALUES (1, '001010000000001', '000000000001', " +
		"'00000000000000000000000000000001', '00000000000000000000000000000002', 1)")

	if err := conn.Close(); err != nil {
		t.Fatalf("failed to close seed legacy db: %v", err)
	}
}

func TestResolveDataDir_FreshInstall(t *testing.T) {
	tmp := t.TempDir()
	dataPath := filepath.Join(tmp, "data")

	dir, err := resolveDataDir(context.Background(), dataPath)
	if err != nil {
		t.Fatalf("resolveDataDir failed: %v", err)
	}

	if dir != dataPath {
		t.Fatalf("expected dir %q, got %q", dataPath, dir)
	}

	info, err := os.Stat(dataPath)
	if err != nil {
		t.Fatalf("stat after fresh install failed: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf("expected %q to be a directory", dataPath)
	}
}

func TestResolveDataDir_AlreadyDirectory(t *testing.T) {
	tmp := t.TempDir()
	dataPath := filepath.Join(tmp, "data")

	if err := os.Mkdir(dataPath, 0o750); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	dir, err := resolveDataDir(context.Background(), dataPath)
	if err != nil {
		t.Fatalf("resolveDataDir failed: %v", err)
	}

	if dir != dataPath {
		t.Fatalf("expected dir %q, got %q", dataPath, dir)
	}
}

func TestResolveDataDir_LegacyMigration(t *testing.T) {
	tmp := t.TempDir()
	dataPath := filepath.Join(tmp, "core.db")

	seedLegacyV8(t, dataPath)

	dir, err := resolveDataDir(context.Background(), dataPath)
	if err != nil {
		t.Fatalf("resolveDataDir failed: %v", err)
	}

	if dir != dataPath {
		t.Fatalf("expected dir %q, got %q", dataPath, dir)
	}

	info, err := os.Stat(dataPath)
	if err != nil {
		t.Fatalf("stat after migration failed: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf("expected %q to be a directory after migration", dataPath)
	}

	for _, name := range []string{SharedDBFilename, LocalDBFilename, embeddedLegacyBackupName} {
		p := filepath.Join(dataPath, name)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %q after migration: %v", p, err)
		}
	}

	// No leftover stage directory or sibling backup.
	if _, err := os.Stat(dataPath + splitStageSuffix); !os.IsNotExist(err) {
		t.Fatalf("expected stage dir to be gone, got err=%v", err)
	}

	if _, err := os.Stat(dataPath + legacyBackupSuffix); !os.IsNotExist(err) {
		t.Fatalf("expected sibling .sqlite.bak to be gone, got err=%v", err)
	}

	// Verify a row from the seeded data made it into shared.db.
	sharedConn, err := openSQLiteConnection(context.Background(), filepath.Join(dataPath, SharedDBFilename), SyncFull)
	if err != nil {
		t.Fatalf("failed to open shared db: %v", err)
	}

	defer func() { _ = sharedConn.Close() }()

	var count int
	if err := sharedConn.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM subscribers").Scan(&count); err != nil {
		t.Fatalf("failed to count subscribers: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 subscriber after migration, got %d", count)
	}
}

func TestResolveDataDir_LegacyMigration_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	dataPath := filepath.Join(tmp, "core.db")

	seedLegacyV8(t, dataPath)

	if _, err := resolveDataDir(context.Background(), dataPath); err != nil {
		t.Fatalf("first resolveDataDir failed: %v", err)
	}

	// Second call should treat dataPath as a directory and return immediately.
	if _, err := resolveDataDir(context.Background(), dataPath); err != nil {
		t.Fatalf("second resolveDataDir failed: %v", err)
	}
}

func TestResolveDataDir_RecoverInterruptedMigration(t *testing.T) {
	tmp := t.TempDir()
	dataPath := filepath.Join(tmp, "core.db")

	// Build a real, valid stage directory by running a full migration on a
	// throwaway path, then capturing its split outputs.
	helperPath := filepath.Join(tmp, "helper.db")
	seedLegacyV8(t, helperPath)

	if _, err := resolveDataDir(context.Background(), helperPath); err != nil {
		t.Fatalf("helper migration failed: %v", err)
	}

	// Simulate the post-step-A state for dataPath: legacy file is at the
	// backup location, the stage directory exists with both DB files, and
	// dataPath itself does not exist.
	stagePath := dataPath + splitStageSuffix
	if err := os.Mkdir(stagePath, 0o750); err != nil {
		t.Fatalf("mkdir stage failed: %v", err)
	}

	for _, name := range []string{SharedDBFilename, LocalDBFilename} {
		src := filepath.Join(helperPath, name)
		dst := filepath.Join(stagePath, name)

		data, err := os.ReadFile(src) // #nosec: G304 — test paths
		if err != nil {
			t.Fatalf("read %q: %v", src, err)
		}

		if err := os.WriteFile(dst, data, 0o600); err != nil {
			t.Fatalf("write %q: %v", dst, err)
		}
	}

	// Drop a fake legacy backup so we can verify it gets moved inside.
	backupPath := dataPath + legacyBackupSuffix
	if err := os.WriteFile(backupPath, []byte("legacy bytes"), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	dir, err := resolveDataDir(context.Background(), dataPath)
	if err != nil {
		t.Fatalf("resolveDataDir failed: %v", err)
	}

	if dir != dataPath {
		t.Fatalf("expected dir %q, got %q", dataPath, dir)
	}

	for _, name := range []string{SharedDBFilename, LocalDBFilename, embeddedLegacyBackupName} {
		if _, err := os.Stat(filepath.Join(dataPath, name)); err != nil {
			t.Fatalf("expected %q inside dataDir: %v", name, err)
		}
	}

	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("expected stage dir to be gone, got err=%v", err)
	}

	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("expected sibling backup to be gone, got err=%v", err)
	}
}

func TestResolveDataDir_RefuseFreshInstallWithOrphanedBackup(t *testing.T) {
	tmp := t.TempDir()
	dataPath := filepath.Join(tmp, "core.db")

	// Backup exists but no stage directory: an interrupted migration we
	// cannot safely recover from automatically.
	if err := os.WriteFile(dataPath+legacyBackupSuffix, []byte("data"), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	if _, err := resolveDataDir(context.Background(), dataPath); err == nil {
		t.Fatal("expected error refusing fresh install in presence of orphaned backup")
	}

	// Confirm we did NOT silently mkdir an empty data directory.
	if _, err := os.Stat(dataPath); !os.IsNotExist(err) {
		t.Fatalf("expected dataPath to remain absent, got err=%v", err)
	}
}

func TestResolveDataDir_NULBytePathRejected(t *testing.T) {
	if _, err := resolveDataDir(context.Background(), "/tmp/has\x00null"); err == nil {
		t.Fatal("expected resolveDataDir to reject NUL byte in path")
	}
}
