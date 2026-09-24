// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/pki"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const ClusterMembersTableName = "cluster_members"

const (
	DrainStateActive   = "active"
	DrainStateDraining = "draining"
	DrainStateDrained  = "drained"
)

const clusterMemberIdentitySchema = 20

func (db *Database) requireIdentitySchemaFor(ctx context.Context, nodeID string) error {
	if db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		return nil
	}

	if _, ok := pki.LegacyNodeID(nodeID); ok {
		return nil
	}

	return ErrMigrationPending
}

const clusterMemberColumnsPreV20 = "&ClusterMember.nodeID, &ClusterMember.apiAddress, &ClusterMember.binaryVersion, &ClusterMember.drainState, &ClusterMember.drainUpdatedAt"

const (
	listClusterMembersStmtStr        = "SELECT &ClusterMember.* FROM %s ORDER BY nodeID ASC"
	listClusterMembersPreV20StmtStr  = "SELECT " + clusterMemberColumnsPreV20 + " FROM %s ORDER BY nodeID ASC"
	getClusterMemberStmtStr          = "SELECT &ClusterMember.* FROM %s WHERE nodeID==$ClusterMember.nodeID"
	getClusterMemberPreV20StmtStr    = "SELECT " + clusterMemberColumnsPreV20 + " FROM %s WHERE nodeID==$ClusterMember.nodeID"
	upsertClusterMemberStmtStr       = "INSERT INTO %s (nodeID, amfPointer, displayName, apiAddress, binaryVersion) VALUES ($ClusterMember.nodeID, $ClusterMember.amfPointer, $ClusterMember.displayName, $ClusterMember.apiAddress, $ClusterMember.binaryVersion) ON CONFLICT(nodeID) DO UPDATE SET amfPointer=excluded.amfPointer, apiAddress=$ClusterMember.apiAddress, binaryVersion=$ClusterMember.binaryVersion"
	upsertClusterMemberPreV20StmtStr = "INSERT INTO %s (nodeID, raftAddress, apiAddress, binaryVersion) VALUES ($ClusterMember.nodeID, '', $ClusterMember.apiAddress, $ClusterMember.binaryVersion) ON CONFLICT(nodeID) DO UPDATE SET apiAddress=$ClusterMember.apiAddress, binaryVersion=$ClusterMember.binaryVersion"
	deleteClusterMemberStmtStr       = "DELETE FROM %s WHERE nodeID==$ClusterMember.nodeID"
	countClusterMembersStmtStr       = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	getDrainStateStmtStr             = "SELECT &ClusterMember.drainState FROM %s WHERE nodeID==$ClusterMember.nodeID"
	setDrainStateStmtStr             = "UPDATE %s SET drainState=$ClusterMember.drainState, drainUpdatedAt=$ClusterMember.drainUpdatedAt WHERE nodeID==$ClusterMember.nodeID"
	setDisplayNameStmtStr            = "UPDATE %s SET displayName=$ClusterMember.displayName WHERE nodeID==$ClusterMember.nodeID"
)

const (
	DefaultAMFPointer = 1
	MaxAMFPointer     = 63
)

type ClusterMember struct {
	NodeID         string `db:"nodeID"`
	AMFPointer     int    `db:"amfPointer"`
	DisplayName    string `db:"displayName"`
	APIAddress     string `db:"apiAddress"`
	BinaryVersion  string `db:"binaryVersion"`
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
			semconv.DBCollectionName(ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "select").Inc()

	var members []ClusterMember

	stmt := db.listClusterMembersStmt
	if !db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		stmt = db.listClusterMembersPreV20Stmt
	}

	err := db.conn().Query(ctx, stmt).GetAll(&members)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return members, nil
}

func (db *Database) GetClusterMember(ctx context.Context, nodeID string) (*ClusterMember, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", ClusterMembersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "select").Inc()

	row := ClusterMember{NodeID: nodeID}

	stmt := db.getClusterMemberStmt
	if !db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		stmt = db.getClusterMemberPreV20Stmt
	}

	err := db.conn().Query(ctx, stmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			recordSpanError(span, err)

			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) UpsertClusterMember(ctx context.Context, member *ClusterMember) error {
	querySummary := fmt.Sprintf("%s %s", "UPSERT", ClusterMembersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			semconv.DBCollectionName(ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "upsert").Inc()

	_, err := opUpsertClusterMember.Invoke(ctx, db, member)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteClusterMember(ctx context.Context, nodeID string) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", ClusterMembersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "delete").Inc()

	_, err := opDeleteClusterMember.Invoke(ctx, db, &nodeIDPayload{Value: pki.NodeID(nodeID)})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func normalizeDrainState(s string) string {
	if s == "" {
		return DrainStateActive
	}

	return s
}

func drainTransitionAllowed(current string, target string) bool {
	switch target {
	case DrainStateDraining:
		return normalizeDrainState(current) == DrainStateActive
	case DrainStateDrained:
		return normalizeDrainState(current) == DrainStateDraining
	case DrainStateActive:
		return normalizeDrainState(current) != DrainStateActive
	default:
		return false
	}
}

func (db *Database) SetDrainState(ctx context.Context, nodeID string, state string) (string, error) {
	if !IsValidDrainState(state) {
		return "", fmt.Errorf("invalid drain state %q", state)
	}

	querySummary := fmt.Sprintf("%s %s", "UPDATE", ClusterMembersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "update").Inc()

	member := &ClusterMember{
		NodeID:         nodeID,
		DrainState:     state,
		DrainUpdatedAt: time.Now().Unix(),
	}

	effective, err := opSetDrainState.Invoke(ctx, db, member)
	if err != nil {
		recordSpanError(span, err)

		return "", err
	}

	return effective, nil
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
			semconv.DBCollectionName(ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countClusterMembersStmt).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

// MaxDisplayNameLength bounds the operator-facing name.
const MaxDisplayNameLength = 63

// SetDisplayName sets a cluster member's operator-facing name. The name
// is free text; lookups and destructive operations take the identity.
func (db *Database) SetDisplayName(ctx context.Context, nodeID string, name string) error {
	if len(name) > MaxDisplayNameLength {
		return fmt.Errorf("display name is %d bytes, over the %d-byte limit", len(name), MaxDisplayNameLength)
	}

	querySummary := fmt.Sprintf("%s %s", "UPDATE", ClusterMembersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "update").Inc()

	member := &ClusterMember{NodeID: nodeID, DisplayName: name}

	if _, err := opSetDisplayName.Invoke(ctx, db, member); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}
