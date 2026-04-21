// Copyright 2026 Ella Networks

package db

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// buildBackupTarGz constructs a gzip-compressed tar with the listed members.
// Members are emitted in order. nil data inserts the manifest with the given
// version.
type tarMember struct {
	name        string
	data        []byte
	typeflag    byte
	overrideLen int64 // when >0, sets the header Size to this value
}

func buildBackupTarGz(t *testing.T, members []tarMember) []byte {
	t.Helper()

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, m := range members {
		typ := m.typeflag
		if typ == 0 {
			typ = tar.TypeReg
		}

		size := int64(len(m.data))
		if m.overrideLen != 0 {
			size = m.overrideLen
		}

		hdr := &tar.Header{
			Name:     m.name,
			Mode:     0o600,
			Size:     size,
			ModTime:  time.Now(),
			Typeflag: typ,
		}

		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}

		if _, err := tw.Write(m.data); err != nil {
			t.Fatalf("write data: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tw close: %v", err)
	}

	if err := gz.Close(); err != nil {
		t.Fatalf("gz close: %v", err)
	}

	return buf.Bytes()
}

func validManifestBytes(t *testing.T, version int) []byte {
	t.Helper()

	b, err := json.Marshal(BackupManifest{Version: version, CreatedAt: time.Now()})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	return b
}

func TestExtractBackupArchive_PathTraversalRejected(t *testing.T) {
	tmp := t.TempDir()

	body := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: validManifestBytes(t, BackupManifestVersion)},
		{name: "../etc/passwd", data: []byte("nope")},
	})

	if err := extractBackupArchive(bytes.NewReader(body), tmp, extractModeOnline); err == nil {
		t.Fatal("expected path traversal rejection")
	} else if !strings.Contains(err.Error(), "invalid tar entry name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractBackupArchive_NonRegularRejected(t *testing.T) {
	tmp := t.TempDir()

	body := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: validManifestBytes(t, BackupManifestVersion)},
		{name: DBFilename, typeflag: tar.TypeSymlink},
	})

	if err := extractBackupArchive(bytes.NewReader(body), tmp, extractModeOnline); err == nil {
		t.Fatal("expected non-regular tar entry rejection")
	} else if !strings.Contains(err.Error(), "unexpected tar entry type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractBackupArchive_DuplicateRejected(t *testing.T) {
	tmp := t.TempDir()

	body := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: validManifestBytes(t, BackupManifestVersion)},
		{name: DBFilename, data: []byte("first")},
		{name: DBFilename, data: []byte("second")},
	})

	if err := extractBackupArchive(bytes.NewReader(body), tmp, extractModeOnline); err == nil {
		t.Fatal("expected duplicate tar entry rejection")
	} else if !strings.Contains(err.Error(), "duplicate tar entry") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractBackupArchive_DuplicateManifestRejected(t *testing.T) {
	tmp := t.TempDir()

	body := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: validManifestBytes(t, BackupManifestVersion)},
		{name: manifestArchiveName, data: validManifestBytes(t, BackupManifestVersion)},
	})

	if err := extractBackupArchive(bytes.NewReader(body), tmp, extractModeOnline); err == nil {
		t.Fatal("expected duplicate manifest rejection")
	}
}

func TestExtractBackupArchive_MissingManifest(t *testing.T) {
	tmp := t.TempDir()

	body := buildBackupTarGz(t, []tarMember{
		{name: DBFilename, data: []byte("data")},
	})

	if err := extractBackupArchive(bytes.NewReader(body), tmp, extractModeOnline); err == nil {
		t.Fatal("expected missing manifest rejection")
	} else if !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractBackupArchive_BadVersionRejected(t *testing.T) {
	tmp := t.TempDir()

	body := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: validManifestBytes(t, BackupManifestVersion+99)},
		{name: DBFilename, data: []byte("data")},
	})

	if err := extractBackupArchive(bytes.NewReader(body), tmp, extractModeOnline); err == nil {
		t.Fatal("expected manifest version rejection")
	} else if !strings.Contains(err.Error(), "manifest version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractBackupArchive_UnknownMemberRejected(t *testing.T) {
	tmp := t.TempDir()

	body := buildBackupTarGz(t, []tarMember{
		{name: manifestArchiveName, data: validManifestBytes(t, BackupManifestVersion)},
		{name: "stranger.db", data: []byte("?")},
	})

	if err := extractBackupArchive(bytes.NewReader(body), tmp, extractModeOnline); err == nil {
		t.Fatal("expected unknown member rejection")
	} else if !strings.Contains(err.Error(), "unexpected backup member") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractBackupArchive_OversizeMemberRejected(t *testing.T) {
	tmp := t.TempDir()

	// A header claiming a size beyond the per-member cap is rejected
	// before any data is read. We hand-craft the bytes because the tar
	// writer refuses to emit a header whose declared size exceeds the
	// data we hand it.
	var rawTar bytes.Buffer

	hdr := &tar.Header{
		Name:     manifestArchiveName,
		Mode:     0o600,
		Size:     int64(maxBackupMemberSize) + 1,
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	}

	tw := tar.NewWriter(&rawTar)
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write header: %v", err)
	}
	// Intentionally do not Close tw — extraction reads only the header.

	var gzBuf bytes.Buffer

	gz := gzip.NewWriter(&gzBuf)
	if _, err := gz.Write(rawTar.Bytes()); err != nil {
		t.Fatalf("gz write: %v", err)
	}

	if err := gz.Close(); err != nil {
		t.Fatalf("gz close: %v", err)
	}

	if err := extractBackupArchive(bytes.NewReader(gzBuf.Bytes()), tmp, extractModeOnline); err == nil {
		t.Fatal("expected oversize entry rejection")
	} else if !strings.Contains(err.Error(), "invalid size") {
		t.Fatalf("unexpected error: %v", err)
	}
}
