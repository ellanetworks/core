package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
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
	upsertClusterMemberStmtStr = "INSERT INTO %s (nodeID, raftAddress, apiAddress, binaryVersion, suffrage, maxSchemaVersion) VALUES ($ClusterMember.nodeID, $ClusterMember.raftAddress, $ClusterMember.apiAddress, $ClusterMember.binaryVersion, $ClusterMember.suffrage, $ClusterMember.maxSchemaVersion) ON CONFLICT(nodeID) DO UPDATE SET raftAddress=$ClusterMember.raftAddress, apiAddress=$ClusterMember.apiAddress, binaryVersion=$ClusterMember.binaryVersion, suffrage=$ClusterMember.suffrage, maxSchemaVersion=$ClusterMember.maxSchemaVersion"
	deleteClusterMemberStmtStr = "DELETE FROM %s WHERE nodeID==$ClusterMember.nodeID"
	countClusterMembersStmtStr = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	setDrainStateStmtStr       = "UPDATE %s SET drainState=$ClusterMember.drainState, drainUpdatedAt=$ClusterMember.drainUpdatedAt WHERE nodeID==$ClusterMember.nodeID"
)

type ClusterMember struct {
	NodeID           int    `db:"nodeID"`
	RaftAddress      string `db:"raftAddress"`
	APIAddress       string `db:"apiAddress"`
	BinaryVersion    string `db:"binaryVersion"`
	Suffrage         string `db:"suffrage"`
	MaxSchemaVersion int    `db:"maxSchemaVersion"`
	DrainState       string `db:"drainState"`
	DrainUpdatedAt   int64  `db:"drainUpdatedAt"`
}

func IsValidDrainState(s string) bool {
	switch s {
	case DrainStateActive, DrainStateDraining, DrainStateDrained:
		return true
	}

	return false
}

func (db *Database) ListClusterMembers(ctx context.Context) ([]ClusterMember, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", ClusterMembersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ClusterMembersTableName),
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
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", ClusterMembersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ClusterMembersTableName),
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
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", ClusterMembersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			attribute.String("db.collection", ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "upsert").Inc()

	_, err := opUpsertClusterMember.Invoke(db, member)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteClusterMember(ctx context.Context, nodeID int) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", ClusterMembersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", ClusterMembersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ClusterMembersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ClusterMembersTableName, "delete").Inc()

	_, err := opDeleteClusterMember.Invoke(db, &intPayload{Value: nodeID})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// SetDrainState persists the drain state for a cluster member. The
// `drainUpdatedAt` column is set to the current unix timestamp. Returns
// ErrNotFound if no row exists for nodeID. The update is replicated
// through the Raft log like any other cluster_members mutation.
func (db *Database) SetDrainState(ctx context.Context, nodeID int, state string) error {
	if !IsValidDrainState(state) {
		return fmt.Errorf("invalid drain state %q", state)
	}

	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", ClusterMembersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", ClusterMembersTableName),
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

	_, err := opSetDrainState.Invoke(db, member)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) CountClusterMembers(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", ClusterMembersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ClusterMembersTableName),
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
