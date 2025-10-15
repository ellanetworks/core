package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const NetworkLogsTableName = "network_logs"

// Structured table (no raw blob). Keep strings NOT NULL with empty defaults to avoid NullString hassle.
const QueryCreateNetworkLogsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp  TEXT NOT NULL,                      -- RFC3339
		protocol      TEXT NOT NULL,
		message_type TEXT NOT NULL,
		direction	TEXT NOT NULL DEFAULT '',       -- inbound|outbound
		local_address TEXT NOT NULL DEFAULT '',
		remote_address TEXT NOT NULL DEFAULT '',
		raw			 BLOB NOT NULL,
		details    TEXT NOT NULL DEFAULT ''
);`

const QueryCreateNetworkLogsIndex = `
	CREATE INDEX IF NOT EXISTS idx_network_logs_protocol ON network_logs (protocol);
	CREATE INDEX IF NOT EXISTS idx_network_logs_timestamp ON network_logs (timestamp);
	CREATE INDEX IF NOT EXISTS idx_network_logs_message_type ON network_logs (message_type);
	CREATE INDEX IF NOT EXISTS idx_network_logs_direction ON network_logs (direction);
	CREATE INDEX IF NOT EXISTS idx_network_logs_local_address ON network_logs (local_address);
	CREATE INDEX IF NOT EXISTS idx_network_logs_remote_address ON network_logs (remote_address);
`

const (
	insertNetworkLogStmt     = "INSERT INTO %s (timestamp, protocol, message_type, direction, local_address, remote_address, raw, details) VALUES ($NetworkLog.timestamp, $NetworkLog.protocol, $NetworkLog.message_type, $NetworkLog.direction, $NetworkLog.local_address, $NetworkLog.remote_address, $NetworkLog.raw, $NetworkLog.details)"
	getNetworkLogByIDStmt    = "SELECT &NetworkLog.* FROM %s WHERE id = $NetworkLog.id"
	deleteOldNetworkLogsStmt = "DELETE FROM %s WHERE timestamp < $cutoffArgs.cutoff"
	deleteAllNetworkLogsStmt = "DELETE FROM %s"
)

const listNetworkLogsPagedFilteredStmt = `
  SELECT &NetworkLog.*
  FROM %s
  WHERE
    ($NetworkLogFilters.protocol      IS NULL OR protocol      = $NetworkLogFilters.protocol)
    AND ($NetworkLogFilters.direction IS NULL OR direction = $NetworkLogFilters.direction)
    AND ($NetworkLogFilters.local_address IS NULL OR local_address = $NetworkLogFilters.local_address)
    AND ($NetworkLogFilters.remote_address IS NULL OR remote_address = $NetworkLogFilters.remote_address)
    AND ($NetworkLogFilters.message_type IS NULL OR message_type     = $NetworkLogFilters.message_type)
    AND ($NetworkLogFilters.timestamp_from  IS NULL OR timestamp >= $NetworkLogFilters.timestamp_from)
    AND ($NetworkLogFilters.timestamp_to    IS NULL OR timestamp <  $NetworkLogFilters.timestamp_to)
  ORDER BY id DESC
  LIMIT $ListArgs.limit
  OFFSET $ListArgs.offset
`

const countNetworkLogsFilteredStmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE
    ($NetworkLogFilters.protocol      IS NULL OR protocol      = $NetworkLogFilters.protocol)
    AND ($NetworkLogFilters.direction IS NULL OR direction = $NetworkLogFilters.direction)
    AND ($NetworkLogFilters.local_address IS NULL OR local_address = $NetworkLogFilters.local_address)
    AND ($NetworkLogFilters.remote_address IS NULL OR remote_address = $NetworkLogFilters.remote_address)
    AND ($NetworkLogFilters.message_type IS NULL OR message_type     = $NetworkLogFilters.message_type)
    AND ($NetworkLogFilters.timestamp_from  IS NULL OR timestamp >= $NetworkLogFilters.timestamp_from)
    AND ($NetworkLogFilters.timestamp_to    IS NULL OR timestamp <  $NetworkLogFilters.timestamp_to)
`

type NetworkLog struct {
	ID            int    `db:"id"`
	Timestamp     string `db:"timestamp"` // store as RFC3339 string; parse in API layer if needed
	Protocol      string `db:"protocol"`
	MessageType   string `db:"message_type"`
	Direction     string `db:"direction"`
	LocalAddress  string `db:"local_address"`
	RemoteAddress string `db:"remote_address"`
	Raw           []byte `db:"raw"`
	Details       string `db:"details"` // JSON or plain text (we store a string)
}

type NetworkLogFilters struct {
	Protocol      *string `db:"protocol"`       // exact match
	Direction     *string `db:"direction"`      // "inbound" | "outbound"
	LocalAddress  *string `db:"local_address"`  // exact match
	RemoteAddress *string `db:"remote_address"` // exact match
	MessageType   *string `db:"message_type"`   // exact match
	TimestampFrom *string `db:"timestamp_from"` // RFC3339 (UTC)
	TimestampTo   *string `db:"timestamp_to"`   // RFC3339 (UTC), exclusive upper bound
}

type zapNetworkJSON struct {
	Timestamp     string `json:"timestamp"`
	Level         string `json:"level"`
	Protocol      string `json:"protocol"`
	MessageType   string `json:"message_type"`
	Direction     string `json:"direction"`
	LocalAddress  string `json:"local_address"`
	RemoteAddress string `json:"remote_address"`
	Raw           []byte `json:"raw"`
	Details       string `json:"details"`
}

func (db *Database) NetworkWriteFunc(ctx context.Context) func([]byte) error {
	return func(b []byte) error {
		return db.InsertNetworkLogJSON(ctx, b)
	}
}

// InsertNetworkLogJSON parses the zap JSON and inserts a structured row.
func (db *Database) InsertNetworkLogJSON(ctx context.Context, raw []byte) error {
	const operation = "INSERT"
	const target = NetworkLogsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	query := fmt.Sprintf(insertNetworkLogStmt, db.networkLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(query),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// Parse incoming JSON
	var z zapNetworkJSON
	if err := json.Unmarshal(raw, &z); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "unmarshal failed")
		return err
	}

	row := NetworkLog{
		Timestamp:     z.Timestamp,
		Protocol:      z.Protocol,
		MessageType:   z.MessageType,
		Direction:     z.Direction,
		LocalAddress:  z.LocalAddress,
		RemoteAddress: z.RemoteAddress,
		Raw:           z.Raw,
		Details:       z.Details,
	}

	stmt, err := sqlair.Prepare(query, NetworkLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, stmt, row).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) ListNetworkLogs(ctx context.Context, page int, perPage int, filters *NetworkLogFilters) ([]NetworkLog, int, error) {
	if filters == nil {
		filters = &NetworkLogFilters{}
	}

	const operation = "SELECT"
	const target = NetworkLogsTableName
	spanName := fmt.Sprintf("%s %s (paged+filtered)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	listSQL := fmt.Sprintf(listNetworkLogsPagedFilteredStmt, db.networkLogsTable)
	countSQL := fmt.Sprintf(countNetworkLogsFilteredStmt, db.networkLogsTable)

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(listSQL),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("page", page),
		attribute.Int("per_page", perPage),
	)

	// Prepare both statements with all the bind models they use
	listStmt, err := sqlair.Prepare(listSQL, ListArgs{}, NetworkLogFilters{}, NetworkLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare list failed")
		return nil, 0, err
	}
	countStmt, err := sqlair.Prepare(countSQL, NetworkLogFilters{}, NumItems{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare count failed")
		return nil, 0, err
	}

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	// Count with filters
	var total NumItems
	if err := db.conn.Query(ctx, countStmt, filters).Get(&total); err != nil && err != sql.ErrNoRows {
		span.RecordError(err)
		span.SetStatus(codes.Error, "count failed")
		return nil, 0, err
	}

	// Rows with filters
	var logs []NetworkLog
	if err := db.conn.Query(ctx, listStmt, args, filters).GetAll(&logs); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, total.Count, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, 0, err
	}

	span.SetStatus(codes.Ok, "")
	return logs, total.Count, nil
}

// DeleteOldNetworkLogs removes logs older than the specified retention period in days.
func (db *Database) DeleteOldNetworkLogs(ctx context.Context, days int) error {
	const operation = "DELETE"
	const target = NetworkLogsTableName
	spanName := fmt.Sprintf("%s %s (retention)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	// Compute UTC cutoff so string comparison works lexicographically for RFC3339
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	stmtStr := fmt.Sprintf(deleteOldNetworkLogsStmt, db.networkLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("retention.days", days),
		attribute.String("retention.cutoff", cutoff),
	)

	stmt, err := sqlair.Prepare(stmtStr, cutoffArgs{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	args := cutoffArgs{Cutoff: cutoff}
	if err := db.conn.Query(ctx, stmt, args).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) ClearNetworkLogs(ctx context.Context) error {
	const operation = "DELETE"
	const target = NetworkLogsTableName
	spanName := fmt.Sprintf("%s %s (all)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(deleteAllNetworkLogsStmt, db.networkLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	stmt, err := sqlair.Prepare(stmtStr)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := db.conn.Query(ctx, stmt).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) GetNetworkLogByID(ctx context.Context, id int) (*NetworkLog, error) {
	const operation = "SELECT"
	const target = NetworkLogsTableName
	spanName := fmt.Sprintf("%s %s (by ID)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	query := fmt.Sprintf(getNetworkLogByIDStmt, db.networkLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(query),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("id", id),
	)

	stmt, err := sqlair.Prepare(query, NetworkLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	log := NetworkLog{ID: id}

	if err := db.conn.Query(ctx, stmt, log).Get(&log); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &log, nil
}
