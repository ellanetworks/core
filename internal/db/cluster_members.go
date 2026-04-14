package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const ClusterMembersTableName = "cluster_members"

const (
	listClusterMembersStmtStr  = "SELECT &ClusterMember.* FROM %s ORDER BY nodeID ASC"
	getClusterMemberStmtStr    = "SELECT &ClusterMember.* FROM %s WHERE nodeID==$ClusterMember.nodeID"
	upsertClusterMemberStmtStr = "INSERT INTO %s (nodeID, raftAddress, apiAddress, protocolVersion, binaryVersion, suffrage) VALUES ($ClusterMember.nodeID, $ClusterMember.raftAddress, $ClusterMember.apiAddress, $ClusterMember.protocolVersion, $ClusterMember.binaryVersion, $ClusterMember.suffrage) ON CONFLICT(nodeID) DO UPDATE SET raftAddress=$ClusterMember.raftAddress, apiAddress=$ClusterMember.apiAddress, protocolVersion=$ClusterMember.protocolVersion, binaryVersion=$ClusterMember.binaryVersion, suffrage=$ClusterMember.suffrage"
	deleteClusterMemberStmtStr = "DELETE FROM %s WHERE nodeID==$ClusterMember.nodeID"
	countClusterMembersStmtStr = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

type ClusterMember struct {
	NodeID          int    `db:"nodeID"`
	RaftAddress     string `db:"raftAddress"`
	APIAddress      string `db:"apiAddress"`
	ProtocolVersion int    `db:"protocolVersion"`
	BinaryVersion   string `db:"binaryVersion"`
	Suffrage        string `db:"suffrage"`
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

	err := db.shared.Query(ctx, db.listClusterMembersStmt).GetAll(&members)
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

	err := db.shared.Query(ctx, db.getClusterMemberStmt, row).Get(&row)
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

	_, err := db.propose(ellaraft.CmdUpsertClusterMember, member)
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

	_, err := db.propose(ellaraft.CmdDeleteClusterMember, &intPayload{Value: nodeID})
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

	err := db.shared.Query(ctx, db.countClusterMembersStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
