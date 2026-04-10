// Copyright 2024 Ella Networks

package db

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// BackupManifestVersion is the on-disk version of the backup tar.gz format.
// Phase 1 ships v1; Phase 2 will introduce a v2 with Raft snapshot fields.
const BackupManifestVersion = 1

// BackupManifest is the JSON document embedded as manifest.json inside every
// backup tar.gz. Phase 1 leaves Raft fields out entirely.
type BackupManifest struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}

// Backup writes a tar.gz archive containing shared.db, local.db, and a
// manifest.json to dst. Both source databases are first VACUUM INTO'd into
// temporary files inside Database.Dir() so that the on-disk image is fully
// consistent and free of WAL pages, then they are streamed into the archive.
// The temp files are removed before returning.
//
// This is a BREAKING CHANGE from the pre-Phase-1 backup format, which wrote
// a single .db file. See spec_ha.md §10 for the authorisation.
func (db *Database) Backup(ctx context.Context, dst io.Writer) error {
	ctx, span := tracer.Start(ctx, "db/backup", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	tmpDir, err := os.MkdirTemp(db.dataDir, "backup-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir for backup: %w", err)
	}

	defer func() { _ = os.RemoveAll(tmpDir) }()

	sharedTmp := filepath.Join(tmpDir, SharedDBFilename)
	localTmp := filepath.Join(tmpDir, LocalDBFilename)

	if _, err := db.shared.PlainDB().ExecContext(ctx, "VACUUM INTO ?", sharedTmp); err != nil {
		return fmt.Errorf("failed to VACUUM INTO shared backup file: %w", err)
	}

	if _, err := db.local.PlainDB().ExecContext(ctx, "VACUUM INTO ?", localTmp); err != nil {
		return fmt.Errorf("failed to VACUUM INTO local backup file: %w", err)
	}

	manifest := BackupManifest{
		Version:   BackupManifestVersion,
		CreatedAt: time.Now().UTC(),
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode manifest: %w", err)
	}

	gzWriter := gzip.NewWriter(dst)
	tarWriter := tar.NewWriter(gzWriter)

	if err := writeTarFile(tarWriter, "manifest.json", manifestBytes, manifest.CreatedAt); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	if err := writeTarFromDisk(tarWriter, sharedTmp, SharedDBFilename); err != nil {
		return fmt.Errorf("failed to write shared.db: %w", err)
	}

	if err := writeTarFromDisk(tarWriter, localTmp, LocalDBFilename); err != nil {
		return fmt.Errorf("failed to write local.db: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return nil
}

func writeTarFile(tw *tar.Writer, name string, data []byte, modTime time.Time) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o600,
		Size:    int64(len(data)),
		ModTime: modTime,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	_, err := tw.Write(data)

	return err
}

func writeTarFromDisk(tw *tar.Writer, srcPath, archiveName string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:    archiveName,
		Mode:    0o600,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(srcPath) // #nosec: G304 — local temp file we just created
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	_, err = io.Copy(tw, f)

	return err
}
