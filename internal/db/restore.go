// Copyright 2024 Ella Networks

package db

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (db *Database) Restore(ctx context.Context, backupFile *os.File) error {
	_, span := tracer.Start(ctx, "DB Restore", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if db.conn == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	if backupFile == nil {
		return fmt.Errorf("backup file is nil")
	}

	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close the database connection: %v", err)
	}

	destinationFile, err := os.Create(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to open destination database file: %v", err)
	}

	_, err = io.Copy(destinationFile, backupFile)
	if err != nil {
		_ = destinationFile.Close()
		return fmt.Errorf("failed to restore database file: %v", err)
	}

	if err := destinationFile.Close(); err != nil {
		return fmt.Errorf("failed to close destination database file: %w", err)
	}

	walFile := db.filepath + "-wal"
	shmFile := db.filepath + "-shm"

	if err := os.Remove(walFile); err != nil && !os.IsNotExist(err) {
		logger.DBLog.Warn("Failed to remove old WAL file", zap.String("file", walFile), zap.Error(err))
	}

	if err := os.Remove(shmFile); err != nil && !os.IsNotExist(err) {
		logger.DBLog.Warn("Failed to remove old SHM file", zap.String("file", shmFile), zap.Error(err))
	}

	sqlConnection, err := openSQLiteConnection(ctx, db.filepath)
	if err != nil {
		return fmt.Errorf("failed to reopen database after restore: %w", err)
	}

	db.conn = sqlair.NewDB(sqlConnection)

	if err := db.PrepareStatements(); err != nil {
		return fmt.Errorf("failed to re-prepare statements after restore: %w", err)
	}

	return nil
}
