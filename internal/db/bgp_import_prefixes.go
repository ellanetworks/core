package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const BGPImportPrefixesTableName = "bgp_import_prefixes"

const (
	listImportPrefixesByPeerStmt   = "SELECT &BGPImportPrefix.* FROM %s WHERE peerID==$BGPImportPrefix.peerID ORDER BY id ASC"
	createImportPrefixStmt         = "INSERT INTO %s (peerID, prefix, maxLength) VALUES ($BGPImportPrefix.peerID, $BGPImportPrefix.prefix, $BGPImportPrefix.maxLength)"
	deleteImportPrefixesByPeerStmt = "DELETE FROM %s WHERE peerID==$BGPImportPrefix.peerID"
)

type BGPImportPrefix struct {
	ID        int    `db:"id"`
	PeerID    int    `db:"peerID"`
	Prefix    string `db:"prefix"`
	MaxLength int    `db:"maxLength"`
}

func (db *Database) ListImportPrefixesByPeer(ctx context.Context, peerID int) ([]BGPImportPrefix, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", BGPImportPrefixesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", BGPImportPrefixesTableName),
			attribute.Int("peerID", peerID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPImportPrefixesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPImportPrefixesTableName, "select").Inc()

	var prefixes []BGPImportPrefix

	err := db.conn().Query(ctx, db.listImportPrefixesByPeerStmt, BGPImportPrefix{PeerID: peerID}).GetAll(&prefixes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return prefixes, nil
}

func (db *Database) SetImportPrefixesForPeer(ctx context.Context, peerID int, prefixes []BGPImportPrefix) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "REPLACE", BGPImportPrefixesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("REPLACE"),
			attribute.String("db.collection", BGPImportPrefixesTableName),
			attribute.Int("peerID", peerID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPImportPrefixesTableName, "replace"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPImportPrefixesTableName, "replace").Inc()

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applySetImportPrefixesForPeer(ctx, &importPrefixesPayload{PeerID: peerID, Prefixes: prefixes})
	}, "SetImportPrefixesForPeer")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
