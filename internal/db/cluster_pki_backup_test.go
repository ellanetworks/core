// Copyright 2026 Ella Networks

package db_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

// TestBackup_ArchiveHasExactlyTwoMembers asserts the minimal archive
// shape: manifest.json + ella.db only. CA signing keys ride along
// inside ella.db now; the archive no longer carries cluster-tls tar
// entries.
func TestBackup_ArchiveHasExactlyTwoMembers(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(tmpDir, "ella.db"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	defer func() { _ = database.Close() }()

	var buf bytes.Buffer
	if err := database.Backup(ctx, &buf); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	gzReader, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}

	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	var names []string

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar next: %v", err)
		}

		names = append(names, hdr.Name)
	}

	if len(names) != 2 {
		t.Fatalf("archive has %d members (%v), want 2 (manifest.json + ella.db)", len(names), names)
	}

	if names[0] != "manifest.json" {
		t.Fatalf("first member = %q, want manifest.json", names[0])
	}

	if names[1] != db.DBFilename {
		t.Fatalf("second member = %q, want %q", names[1], db.DBFilename)
	}
}
