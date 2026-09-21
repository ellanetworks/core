// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

const followerTestNodeID = "11111111-1111-1111-1111-111111111111"

func newFollowerDatabase(t *testing.T) *Database {
	t.Helper()

	cfg := ellaraft.FastTestConfig()
	cfg.Enabled = true
	cfg.Bootstrap = false
	cfg.RaftID = followerTestNodeID
	cfg.BindAddress = "127.0.0.1:0"
	cfg.AdvertiseAddress = "127.0.0.1:0"

	database, err := NewDatabase(t.Context(), filepath.Join(t.TempDir(), "ella.db"), cfg)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if database.IsLeader() {
		t.Fatal("test node became leader; it must stay a follower for this test")
	}

	return database
}

func readBackupArchive(t *testing.T, r io.Reader) (BackupManifest, []string) {
	t.Helper()

	gzReader, err := gzip.NewReader(r)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}

	defer func() { _ = gzReader.Close() }()

	var (
		manifest BackupManifest
		names    []string
	)

	tarReader := tar.NewReader(gzReader)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar next: %v", err)
		}

		names = append(names, hdr.Name)

		if hdr.Name == manifestArchiveName {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				t.Fatalf("read manifest: %v", err)
			}

			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatalf("decode manifest: %v", err)
			}
		}
	}

	return manifest, names
}

func TestBackupSucceedsOnFollower(t *testing.T) {
	database := newFollowerDatabase(t)

	var buf bytes.Buffer

	start := time.Now()

	if err := database.Backup(t.Context(), &buf); err != nil {
		t.Fatalf("Backup on a follower: %v", err)
	}

	elapsed := time.Since(start)

	if elapsed > 10*time.Second {
		t.Fatalf("backup took %s; the Raft barrier must not block on a follower", elapsed)
	}

	manifest, names := readBackupArchive(t, &buf)

	if len(names) != 2 || names[0] != manifestArchiveName || names[1] != DBFilename {
		t.Fatalf("archive members = %v, want [%s %s]", names, manifestArchiveName, DBFilename)
	}

	if manifest.Version != BackupManifestVersion {
		t.Fatalf("manifest version = %d, want %d", manifest.Version, BackupManifestVersion)
	}

	if string(manifest.SourceNodeID) != followerTestNodeID {
		t.Fatalf("manifest source node = %q, want %q", manifest.SourceNodeID, followerTestNodeID)
	}
}

func TestBackupOnFollowerIsRestorable(t *testing.T) {
	database := newFollowerDatabase(t)

	var buf bytes.Buffer
	if err := database.Backup(t.Context(), &buf); err != nil {
		t.Fatalf("Backup on a follower: %v", err)
	}

	dest := filepath.Join(t.TempDir(), DBFilename)
	if err := extractBackupArchive(bytes.NewReader(buf.Bytes()), dest); err != nil {
		t.Fatalf("a follower's backup did not survive extraction: %v", err)
	}
}
