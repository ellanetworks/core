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

// BackupManifestVersion is the on-disk version of the backup tar.gz
// format. Version 1 carries manifest.json and ella.db only. The CA
// signing keys now live in the replicated DB, so the archive captures
// them automatically without special-case tar entries.
const BackupManifestVersion = 1

// BackupManifest is the JSON document embedded as manifest.json inside every
// backup tar.gz.
type BackupManifest struct {
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	RaftIndex    uint64    `json:"raft_index"`
	RaftTerm     uint64    `json:"raft_term"`
	SourceNodeID int       `json:"source_node_id"`
}

// Backup writes a tar.gz archive (manifest.json, ella.db) to dst. The source
// database is VACUUM INTO'd into a temp file first to produce a consistent,
// WAL-free image before streaming.
func (db *Database) Backup(ctx context.Context, dst io.Writer) error {
	ctx, span := tracer.Start(ctx, "db/backup", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	tmpDir, err := os.MkdirTemp(db.dataDir, "backup-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir for backup: %w", err)
	}

	defer func() { _ = os.RemoveAll(tmpDir) }()

	if db.raftManager != nil {
		if err := db.raftManager.Barrier(30 * time.Second); err != nil {
			return fmt.Errorf("raft barrier before backup: %w", err)
		}
	}

	dbTmp := filepath.Join(tmpDir, DBFilename)

	if _, err := db.conn().PlainDB().ExecContext(ctx, "VACUUM INTO ?", dbTmp); err != nil {
		return fmt.Errorf("failed to VACUUM INTO backup file: %w", err)
	}

	manifest := BackupManifest{
		Version:      BackupManifestVersion,
		CreatedAt:    time.Now().UTC(),
		SourceNodeID: db.NodeID(),
	}

	if db.raftManager != nil {
		manifest.RaftIndex = db.raftManager.AppliedIndex()

		stats := db.raftManager.Stats()
		if v, ok := stats["term"]; ok {
			_, _ = fmt.Sscanf(v, "%d", &manifest.RaftTerm)
		}
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

	if err := writeTarFromDisk(tarWriter, dbTmp, DBFilename); err != nil {
		return fmt.Errorf("failed to write %s: %w", DBFilename, err)
	}

	// The archive now carries all cluster secrets (CA signing keys,
	// HMAC key, operator secrets) inside ella.db. Warn on every
	// invocation so operators who don't read the docs don't ship
	// unencrypted archives to cloud storage.
	fmt.Fprintln(os.Stderr, "warning: backup archive contains cluster signing keys; store and transfer encrypted")

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
