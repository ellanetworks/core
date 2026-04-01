// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const IPLeasesTableName = "ip_leases"

const (
	createLeaseStmt              = "INSERT INTO %s (poolID, address, imsi, sessionID, type, createdAt) VALUES ($IPLease.poolID, $IPLease.address, $IPLease.imsi, $IPLease.sessionID, $IPLease.type, $IPLease.createdAt)"
	getDynamicLeaseStmt          = "SELECT &IPLease.* FROM %s WHERE poolID==$IPLease.poolID AND imsi==$IPLease.imsi AND type='dynamic'"
	getLeaseBySessionStmt        = "SELECT &IPLease.* FROM %s WHERE poolID==$IPLease.poolID AND sessionID==$IPLease.sessionID AND imsi==$IPLease.imsi"
	updateLeaseSessionStmt       = "UPDATE %s SET sessionID=$IPLease.sessionID WHERE id==$IPLease.id"
	deleteLeaseStmt              = "DELETE FROM %s WHERE id==$IPLease.id AND type='dynamic'"
	deleteAllDynamicLeasesStmt   = "DELETE FROM %s WHERE type='dynamic'"
	listActiveLeasesStmt         = "SELECT &IPLease.* FROM %s WHERE sessionID IS NOT NULL"
	listLeasesByPoolStmt         = "SELECT &IPLease.* FROM %s WHERE poolID==$IPLease.poolID"
	listLeaseAddressesByPoolStmt = "SELECT &IPLease.address FROM %s WHERE poolID==$IPLease.poolID ORDER BY address"
	countLeasesByPoolStmt        = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE poolID==$IPLease.poolID"
	countActiveLeasesStmt        = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE sessionID IS NOT NULL"
	countLeasesByIMSIStmt        = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE imsi==$IPLease.imsi"
	listLeasesByPoolPageStmt     = "SELECT &IPLease.*, COUNT(*) OVER() AS &NumItems.count FROM %s WHERE poolID==$IPLease.poolID ORDER BY address LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	listAllLeasesStmt            = "SELECT &IPLease.* FROM %s"
)

// IPLease represents a row in the ip_leases table.
type IPLease struct {
	ID        int    `db:"id"`
	PoolID    int    `db:"poolID"`
	Address   string `db:"address"`
	IMSI      string `db:"imsi"`
	SessionID *int   `db:"sessionID"`
	Type      string `db:"type"`
	CreatedAt int64  `db:"createdAt"`
}

// CreateLease inserts a new IP lease row. Returns ErrAlreadyExists if the
// (poolID, address) unique constraint is violated.
func (db *Database) CreateLease(ctx context.Context, lease *IPLease) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.createLeaseStmt, lease).Run()
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")

			return ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// GetDynamicLease returns the dynamic lease for (poolID, imsi), or ErrNotFound.
func (db *Database) GetDynamicLease(ctx context.Context, poolID int, imsi string) (*IPLease, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (dynamic)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	row := IPLease{PoolID: poolID, IMSI: imsi}

	err := db.conn.Query(ctx, db.getDynamicLeaseStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

// GetLeaseBySession returns the lease matching (poolID, sessionID, imsi), or ErrNotFound.
func (db *Database) GetLeaseBySession(ctx context.Context, poolID int, sessionID int, imsi string) (*IPLease, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by session)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	row := IPLease{PoolID: poolID, SessionID: &sessionID, IMSI: imsi}

	err := db.conn.Query(ctx, db.getLeaseBySessionStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

// UpdateLeaseSession sets the sessionID on an existing lease.
func (db *Database) UpdateLeaseSession(ctx context.Context, leaseID int, sessionID int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (session)", "UPDATE", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "update").Inc()

	lease := IPLease{ID: leaseID, SessionID: &sessionID}

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.updateLeaseSessionStmt, lease).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// DeleteDynamicLease deletes a dynamic lease by ID.
func (db *Database) DeleteDynamicLease(ctx context.Context, leaseID int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "delete").Inc()

	err := db.conn.Query(ctx, db.deleteLeaseStmt, IPLease{ID: leaseID}).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// DeleteAllDynamicLeases removes all dynamic leases. Called on startup to clean
// up stale leases from a previous process lifetime. Static leases are preserved.
func (db *Database) DeleteAllDynamicLeases(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"DeleteAllDynamicLeases",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "delete").Inc()

	err := db.conn.Query(ctx, db.deleteAllDynamicLeasesStmt).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("failed to delete all dynamic leases: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// ListActiveLeases returns all leases with a non-NULL sessionID (dynamic + active static).
func (db *Database) ListActiveLeases(ctx context.Context) ([]IPLease, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (active)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	var leases []IPLease

	err := db.conn.Query(ctx, db.listActiveLeasesStmt).GetAll(&leases)
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

	return leases, nil
}

// listAllLeases returns every lease row. Used only by the support bundle export.
func (db *Database) listAllLeases(ctx context.Context) ([]IPLease, error) {
	var leases []IPLease

	err := db.conn.Query(ctx, db.listAllLeasesStmt).GetAll(&leases)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return leases, nil
}

// ListLeasesByPool returns all leases (dynamic + static) for a given pool.
func (db *Database) ListLeasesByPool(ctx context.Context, poolID int) ([]IPLease, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by pool)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	var leases []IPLease

	err := db.conn.Query(ctx, db.listLeasesByPoolStmt, IPLease{PoolID: poolID}).GetAll(&leases)
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

	return leases, nil
}

// ListLeasesByPoolPage returns a page of leases for a pool, ordered by address,
// along with the total count. The page parameter is 1-based.
func (db *Database) ListLeasesByPoolPage(ctx context.Context, poolID int, page, perPage int) ([]IPLease, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged by pool)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	var leases []IPLease

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	err := db.conn.Query(ctx, db.listLeasesByPoolPageStmt, args, IPLease{PoolID: poolID}).GetAll(&leases, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountLeasesByPool(ctx, poolID)
			if countErr != nil {
				return nil, 0, fmt.Errorf("count fallback failed: %w", countErr)
			}

			return nil, fallbackCount, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	span.SetStatus(codes.Ok, "")

	return leases, count, nil
}

// ListLeaseAddressesByPool returns sorted addresses for all leases in a pool.
// Used by the allocator to find free offsets via merge-scan.
func (db *Database) ListLeaseAddressesByPool(ctx context.Context, poolID int) ([]string, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (addresses by pool)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	var leases []IPLease

	err := db.conn.Query(ctx, db.listLeaseAddressesByPoolStmt, IPLease{PoolID: poolID}).GetAll(&leases)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	addresses := make([]string, 0, len(leases))
	for _, l := range leases {
		addresses = append(addresses, l.Address)
	}

	span.SetStatus(codes.Ok, "")

	return addresses, nil
}

// CountLeasesByPool returns the total number of leases in a pool.
func (db *Database) CountLeasesByPool(ctx context.Context, poolID int) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (count by pool)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countLeasesByPoolStmt, IPLease{PoolID: poolID}).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

// CountActiveLeases returns the total number of active leases (sessionID IS NOT NULL).
func (db *Database) CountActiveLeases(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (count active)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countActiveLeasesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

// CountLeasesByIMSI returns the total number of leases (all types) for a subscriber.
func (db *Database) CountLeasesByIMSI(ctx context.Context, imsi string) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (count by imsi)", "SELECT", IPLeasesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", IPLeasesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPLeasesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPLeasesTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countLeasesByIMSIStmt, IPLease{IMSI: imsi}).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
