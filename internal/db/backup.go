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
// v2 adds the optional cluster-tls/root.key and cluster-tls/intermediate.key
// entries needed for disaster-recovery of a PKI-destroyed cluster.
const BackupManifestVersion = 2

// ClusterTLSDir is the subdirectory under <dataDir> that holds the
// issuer's private key material. Backup and restore know this name so a
// bundle can carry the keys across a total voter filesystem loss.
const ClusterTLSDir = "cluster-tls"

// Tar entry names for the CA private keys. Paired with ClusterTLSDir on
// the filesystem side.
const (
	backupRootKeyName         = "cluster-tls/root.key"
	backupIntermediateKeyName = "cluster-tls/intermediate.key"
)

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

	// Include the issuer's private keys when present. Their absence on
	// a pre-PKI node is non-fatal; the bundle just can't be used for
	// DR of a PKI-destroyed cluster in that case.
	clusterTLS := filepath.Join(db.dataDir, ClusterTLSDir)

	for _, entry := range []struct {
		diskName, archiveName string
	}{
		{"root.key", backupRootKeyName},
		{"intermediate.key", backupIntermediateKeyName},
	} {
		p := filepath.Join(clusterTLS, entry.diskName)
		if _, err := os.Stat(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return fmt.Errorf("stat %s: %w", p, err)
		}

		if err := writeTarFromDisk(tarWriter, p, entry.archiveName); err != nil {
			return fmt.Errorf("failed to write %s: %w", entry.archiveName, err)
		}
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
