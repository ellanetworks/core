// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const ClusterMembersTableName = "cluster_members"

const (
	DrainStateActive   = "active"
	DrainStateDraining = "draining"
	DrainStateDrained  = "drained"
)

const (
	listClusterMembersStmtStr  = "SELECT &ClusterMember.* FROM %s ORDER BY nodeID ASC"
	getClusterMemberStmtStr    = "SELECT &ClusterMember.* FROM %s WHERE nodeID==$ClusterMember.nodeID"
	upsertClusterMemberStmtStr = "INSERT INTO %s (nodeID, raftAddress, apiAddress, binaryVersion, suffrage) VALUES ($ClusterMember.nodeID, $ClusterMember.raftAddress, $ClusterMember.apiAddress, $ClusterMember.binaryVersion, $ClusterMember.suffrage) ON CONFLICT(nodeID) DO UPDATE SET raftAddress=$ClusterMember.raftAddress, apiAddress=$ClusterMember.apiAddress, binaryVersion=$ClusterMember.binaryVersion, suffrage=$ClusterMember.suffrage"
	deleteClusterMemberStmtStr = "DELETE FROM %s WHERE nodeID==$ClusterMember.nodeID"
	countClusterMembersStmtStr = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	setDrainStateStmtStr       = "UPDATE %s SET drainState=$ClusterMember.drainState, drainUpdatedAt=$ClusterMember.drainUpdatedAt WHERE nodeID==$ClusterMember.nodeID"
)

type ClusterMember struct {
	NodeID         int    `db:"nodeID"`
	RaftAddress    string `db:"raftAddress"`
	APIAddress     string `db:"apiAddress"`
	BinaryVersion  string `db:"binaryVersion"`
	Suffrage       string `db:"suffrage"`
	DrainState     string `db:"drainState"`
	DrainUpdatedAt int64  `db:"drainUpdatedAt"`
}

func IsValidDrainState(s string) bool {
	switch s {
	case DrainStateActive, DrainStateDraining, DrainStateDrained:
		return true
	}

	return false
}

func normalizeDrainState(s string) string {
	if s == "" {
		return DrainStateActive
	}

	return s
}

type setDrainStatePayload struct {
	NodeID         int
	DrainState     string
	DrainUpdatedAt int64
	ExpectFrom     []string `json:",omitempty"`
}

func (db *Database) ListClusterMembers(ctx context.Context) ([]ClusterMember, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", ClusterMembersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection.name", ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "select").Inc()

	var members []ClusterMember

	err := db.conn().Query(ctx, db.listClusterMembersStmt).GetAll(&members)
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

	return members, nil
}

func (db *Database) GetClusterMember(ctx context.Context, nodeID int) (*ClusterMember, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", ClusterMembersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection.name", ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "select").Inc()

	row := ClusterMember{NodeID: nodeID}

	err := db.conn().Query(ctx, db.getClusterMemberStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.RecordError(err)
			span.SetStatus(codes.Error, "not found")

			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) UpsertClusterMember(ctx context.Context, member *ClusterMember) error {
	querySummary := fmt.Sprintf("%s %s", "UPSERT", ClusterMembersTableName)

	_, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			attribute.String("db.collection.name", ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "upsert").Inc()

	_, err := opUpsertClusterMember.Invoke(ctx, db, member)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteClusterMember(ctx context.Context, nodeID int) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", ClusterMembersTableName)

	_, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection.name", ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "delete").Inc()

	_, err := opDeleteClusterMember.Invoke(ctx, db, &intPayload{Value: nodeID})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) SetDrainStateIf(ctx context.Context, nodeID int, from []string, state string) (string, error) {
	if !IsValidDrainState(state) {
		return "", fmt.Errorf("invalid drain state %q", state)
	}

	querySummary := fmt.Sprintf("%s %s", "UPDATE", ClusterMembersTableName)

	_, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection.name", ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "update").Inc()

	payload := &setDrainStatePayload{
		NodeID:         nodeID,
		DrainState:     state,
		DrainUpdatedAt: time.Now().Unix(),
		ExpectFrom:     from,
	}

	settled, err := opSetDrainState.Invoke(ctx, db, payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return "", err
	}

	span.SetStatus(codes.Ok, "")

	if settled == "" {
		logger.From(ctx, logger.DBLog).Warn(
			"Drain state compare skipped: the leader predates it and applied the transition unconditionally",
			zap.Int("node_id", nodeID),
			zap.String("state", state),
		)

		settled = state
	}

	return settled, nil
}

func (db *Database) CountClusterMembers(ctx context.Context) (int, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", ClusterMembersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection.name", ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countClusterMembersStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
