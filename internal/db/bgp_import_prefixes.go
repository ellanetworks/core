// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
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
	querySummary := fmt.Sprintf("%s %s", "SELECT", BGPImportPrefixesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(BGPImportPrefixesTableName),
			attribute.Int("bgp.peer_id", peerID),
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
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return prefixes, nil
}

func (db *Database) SetImportPrefixesForPeer(ctx context.Context, peerID int, prefixes []BGPImportPrefix) error {
	querySummary := fmt.Sprintf("%s %s", "REPLACE", BGPImportPrefixesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("REPLACE"),
			semconv.DBCollectionName(BGPImportPrefixesTableName),
			attribute.Int("bgp.peer_id", peerID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPImportPrefixesTableName, "replace"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPImportPrefixesTableName, "replace").Inc()

	tx, err := db.BeginTransaction(ctx)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	defer func() { _ = tx.Rollback() }()

	if err := tx.tx.Query(ctx, db.deleteImportPrefixesByPeerStmt, BGPImportPrefix{PeerID: peerID}).Run(); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("delete existing prefixes: %w", err)
	}

	for _, prefix := range prefixes {
		prefix.PeerID = peerID

		if err := tx.tx.Query(ctx, db.createImportPrefixStmt, prefix).Run(); err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("insert prefix: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)

		return err
	}

	db.publishOpTopics([]Topic{TopicBGPPeers})

	return nil
}
