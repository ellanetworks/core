// Copyright 2024 Ella Networks

package db

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/trace"
)

func (db *Database) Backup(ctx context.Context, destinationFile *os.File) error {
	ctx, span := tracer.Start(ctx, "VACUUM", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	_, err := db.conn.PlainDB().ExecContext(ctx, "VACUUM INTO ?", destinationFile.Name())
	if err != nil {
		return fmt.Errorf("failed to VACUUM INTO: %w", err)
	}

	return nil
}
