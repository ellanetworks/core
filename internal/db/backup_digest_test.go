// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func sha256Bytes(b []byte) (string, error) {
	h := sha256.New()
	if _, err := h.Write(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func digestManifestBytes(t *testing.T, version int, sum string) []byte {
	t.Helper()

	b, err := json.Marshal(BackupManifest{Version: version, CreatedAt: time.Now(), DBSHA256: sum})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	return b
}

func TestExtractBackupArchive_DigestMismatchRejected(t *testing.T) {
	payload := []byte("SQLite format 3\x00 pretend database")

	archive := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: digestManifestBytes(t, 1, strings.Repeat("a", 64))},
		{name: DBFilename, data: payload},
	})

	err := extractBackupArchive(bytesReader(archive), filepath.Join(t.TempDir(), DBFilename))
	if err == nil {
		t.Fatal("digest mismatch was accepted")
	}

	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractBackupArchive_DigestMatchAccepted(t *testing.T) {
	payload := []byte("SQLite format 3\x00 pretend database")

	sum, err := sha256Bytes(payload)
	if err != nil {
		t.Fatalf("hash payload: %v", err)
	}

	archive := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: digestManifestBytes(t, 1, sum)},
		{name: DBFilename, data: payload},
	})

	if err := extractBackupArchive(bytesReader(archive), filepath.Join(t.TempDir(), DBFilename)); err != nil {
		t.Fatalf("matching digest was rejected: %v", err)
	}
}

func TestExtractBackupArchive_ManifestWithoutDigestStillAccepted(t *testing.T) {
	payload := []byte("SQLite format 3\x00 pretend database")

	archive := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: digestManifestBytes(t, 1, "")},
		{name: DBFilename, data: payload},
	})

	if err := extractBackupArchive(bytesReader(archive), filepath.Join(t.TempDir(), DBFilename)); err != nil {
		t.Fatalf("pre-digest archive was rejected: %v", err)
	}
}
