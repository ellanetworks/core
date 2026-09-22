// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func goodArchive(t *testing.T) []byte {
	t.Helper()

	payload := []byte("SQLite format 3\x00 pretend database payload for gzip testing")
	h := sha256.New()
	h.Write(payload)
	man, _ := json.Marshal(BackupManifest{Version: 1, CreatedAt: time.Now(), DBSHA256: hex.EncodeToString(h.Sum(nil))})

	return buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: man},
		{name: DBFilename, data: payload},
	})
}

func TestExtractBackupArchive_CorruptGzipChecksumRejected(t *testing.T) {
	a := goodArchive(t)
	if err := extractBackupArchive(bytes.NewReader(a), filepath.Join(t.TempDir(), DBFilename)); err != nil {
		t.Fatalf("baseline archive rejected: %v", err)
	}
	// gzip trailer is the last 8 bytes: CRC32 then ISIZE. Corrupt the CRC.
	bad := make([]byte, len(a))
	copy(bad, a)

	bad[len(bad)-8] ^= 0xFF
	if err := extractBackupArchive(bytes.NewReader(bad), filepath.Join(t.TempDir(), DBFilename)); err == nil {
		t.Fatal("corrupted gzip checksum was accepted")
	}
}

func TestExtractBackupArchive_TrailingDataRejected(t *testing.T) {
	payload := []byte("SQLite format 3\x00 pretend database payload for gzip testing")
	h := sha256.New()
	h.Write(payload)
	man, _ := json.Marshal(BackupManifest{Version: 1, CreatedAt: time.Now(), DBSHA256: hex.EncodeToString(h.Sum(nil))})

	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)

	tw := tar.NewWriter(zw)
	for _, m := range []struct {
		n string
		d []byte
	}{{manifestArchiveName, man}, {DBFilename, payload}} {
		_ = tw.WriteHeader(&tar.Header{Name: m.n, Mode: 0o600, Size: int64(len(m.d)), ModTime: time.Now(), Typeflag: tar.TypeReg})
		_, _ = tw.Write(m.d)
	}

	_ = tw.Close()
	// Append arbitrary bytes INSIDE the gzip stream, after the tar end marker.
	_, _ = zw.Write(bytes.Repeat([]byte("SMUGGLED"), 1024))
	_ = zw.Close()

	if err := extractBackupArchive(bytes.NewReader(buf.Bytes()), filepath.Join(t.TempDir(), DBFilename)); err == nil {
		t.Fatal("data smuggled after the tar end-marker was accepted")
	}
}

func TestExtractBackupArchive_ZeroPaddingTolerated(t *testing.T) {
	payload := []byte("SQLite format 3\x00 pretend database payload for gzip testing")
	h := sha256.New()
	h.Write(payload)
	man, _ := json.Marshal(BackupManifest{Version: 1, CreatedAt: time.Now(), DBSHA256: hex.EncodeToString(h.Sum(nil))})

	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)

	tw := tar.NewWriter(zw)
	for _, m := range []struct {
		n string
		d []byte
	}{{manifestArchiveName, man}, {DBFilename, payload}} {
		_ = tw.WriteHeader(&tar.Header{Name: m.n, Mode: 0o600, Size: int64(len(m.d)), ModTime: time.Now(), Typeflag: tar.TypeReg})
		_, _ = tw.Write(m.d)
	}

	_ = tw.Close()
	_, _ = zw.Write(make([]byte, 10240))
	_ = zw.Close()

	if err := extractBackupArchive(bytes.NewReader(buf.Bytes()), filepath.Join(t.TempDir(), DBFilename)); err != nil {
		t.Fatalf("GNU-tar-style zero padding rejected: %v", err)
	}
}

func TestExtractBackupArchive_TruncatedArchiveRejected(t *testing.T) {
	a := goodArchive(t)
	if err := extractBackupArchive(bytes.NewReader(a[:len(a)-4]), filepath.Join(t.TempDir(), DBFilename)); err == nil {
		t.Fatal("truncated archive was accepted")
	}
}
