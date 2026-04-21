// Copyright 2026 Ella Networks

package db_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

// TestBackup_IncludesCAKeys verifies that db.Backup bundles the cluster-tls
// key files when they exist on the backing-up node's disk.
func TestBackup_IncludesCAKeys(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(tmpDir, "ella.db"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	defer func() { _ = database.Close() }()

	// Drop synthetic key files into <dataDir>/cluster-tls/.
	clusterTLS := filepath.Join(database.Dir(), db.ClusterTLSDir)
	if err := os.MkdirAll(clusterTLS, 0o700); err != nil {
		t.Fatal(err)
	}

	rootKey := []byte("ROOT-KEY-PEM")
	intKey := []byte("INT-KEY-PEM")

	if err := os.WriteFile(filepath.Join(clusterTLS, "root.key"), rootKey, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(clusterTLS, "intermediate.key"), intKey, 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := database.Backup(ctx, &buf); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	names, contents := readBackup(t, buf.Bytes())

	mustHave := map[string][]byte{
		"manifest.json":                nil, // content checked separately
		"ella.db":                      nil,
		"cluster-tls/root.key":         rootKey,
		"cluster-tls/intermediate.key": intKey,
	}

	for name, want := range mustHave {
		got, ok := contents[name]
		if !ok {
			t.Fatalf("backup missing entry %q; present: %v", name, names)
		}

		if want != nil && !bytes.Equal(got, want) {
			t.Fatalf("entry %q content mismatch", name)
		}
	}

	// Manifest must be v2.
	var m db.BackupManifest
	if err := json.Unmarshal(contents["manifest.json"], &m); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}

	if m.Version != db.BackupManifestVersion {
		t.Fatalf("manifest.Version = %d, want %d", m.Version, db.BackupManifestVersion)
	}
}

// TestBackup_KeysOptional verifies that Backup succeeds even when the
// cluster-tls directory is absent (pre-PKI node).
func TestBackup_KeysOptional(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(tmpDir, "ella.db"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = database.Close() }()

	var buf bytes.Buffer
	if err := database.Backup(ctx, &buf); err != nil {
		t.Fatalf("Backup without keys must succeed: %v", err)
	}

	names, _ := readBackup(t, buf.Bytes())
	for _, n := range names {
		if n == "cluster-tls/root.key" || n == "cluster-tls/intermediate.key" {
			t.Fatalf("keyless backup unexpectedly included %q", n)
		}
	}
}

// TestExtractForDR_RoundTrip verifies the backup→DR-extract cycle:
// synthetic keys go in, the same bytes come out under the destination
// cluster-tls/.
func TestExtractForDR_RoundTrip(t *testing.T) {
	ctx := context.Background()
	srcDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(srcDir, "ella.db"))
	if err != nil {
		t.Fatal(err)
	}

	clusterTLS := filepath.Join(database.Dir(), db.ClusterTLSDir)
	if err := os.MkdirAll(clusterTLS, 0o700); err != nil {
		t.Fatal(err)
	}

	rootKey := []byte("ROOT-KEY-PEM")
	intKey := []byte("INT-KEY-PEM")
	_ = os.WriteFile(filepath.Join(clusterTLS, "root.key"), rootKey, 0o600)
	_ = os.WriteFile(filepath.Join(clusterTLS, "intermediate.key"), intKey, 0o600)

	var buf bytes.Buffer
	if err := database.Backup(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	_ = database.Close()

	bundlePath := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if err := os.WriteFile(bundlePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	// Fresh destination.
	destDir := t.TempDir()

	if err := db.ExtractForDR(bundlePath, destDir); err != nil {
		t.Fatalf("ExtractForDR: %v", err)
	}

	// ella.db present.
	if _, err := os.Stat(filepath.Join(destDir, "ella.db")); err != nil {
		t.Fatalf("ella.db missing after extract: %v", err)
	}

	// Keys present with matching content.
	gotRoot, err := os.ReadFile(filepath.Join(destDir, db.ClusterTLSDir, "root.key"))
	if err != nil {
		t.Fatalf("read extracted root.key: %v", err)
	}

	if !bytes.Equal(gotRoot, rootKey) {
		t.Fatal("root.key content mismatch")
	}

	gotInt, err := os.ReadFile(filepath.Join(destDir, db.ClusterTLSDir, "intermediate.key"))
	if err != nil {
		t.Fatalf("read extracted intermediate.key: %v", err)
	}

	if !bytes.Equal(gotInt, intKey) {
		t.Fatal("intermediate.key content mismatch")
	}
}

// TestExtractForDR_V1BundleRejected verifies that a bundle lacking key
// entries fails in DR mode with a clear error.
func TestExtractForDR_V1BundleRejected(t *testing.T) {
	tmpDir := t.TempDir()

	manifest, _ := json.Marshal(db.BackupManifest{
		Version:   1,
		CreatedAt: time.Now().UTC(),
	})

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	_ = tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(manifest))})
	_, _ = tw.Write(manifest)

	dbBytes := []byte("SQLite format 3\x00")
	_ = tw.WriteHeader(&tar.Header{Name: "ella.db", Mode: 0o600, Size: int64(len(dbBytes))})
	_, _ = tw.Write(dbBytes)

	_ = tw.Close()
	_ = gz.Close()

	bundlePath := filepath.Join(tmpDir, "v1.bundle")
	if err := os.WriteFile(bundlePath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	err := db.ExtractForDR(bundlePath, t.TempDir())
	if err == nil {
		t.Fatal("v1 bundle must be rejected in DR mode")
	}
}

func readBackup(t *testing.T, raw []byte) ([]string, map[string][]byte) {
	t.Helper()

	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(gz)

	names := []string{}
	contents := map[string][]byte{}

	for {
		h, err := tr.Next()
		if err != nil {
			break
		}

		data, _ := readAllBounded(tr, h.Size)
		names = append(names, h.Name)
		contents[h.Name] = data
	}

	return names, contents
}

func readAllBounded(r *tar.Reader, n int64) ([]byte, error) {
	buf := make([]byte, n)
	_, err := r.Read(buf)

	return buf, err
}
