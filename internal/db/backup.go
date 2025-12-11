// Copyright 2024 Ella Networks

package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/trace"
)

func (db *Database) Backup(ctx context.Context, destinationFile *os.File) error {
	ctx, span := tracer.Start(ctx, "VACUUM", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if db.conn == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	if err := os.MkdirAll(filepath.Dir(destinationFile.Name()), 0o755); err != nil {
		return fmt.Errorf("failed to create backup dir: %w", err)
	}

	if _, err := db.conn.PlainDB().ExecContext(ctx, "VACUUM INTO ?", destinationFile.Name()); err != nil {
		return fmt.Errorf("VACUUM INTO failed: %w", err)
	}

	return nil
}
