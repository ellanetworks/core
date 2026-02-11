// Copyright 2024 Ella Networks

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
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const PoliciesTableName = "policies"

const QueryCreatePoliciesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL UNIQUE,

		bitrateUplink TEXT NOT NULL,
		bitrateDownlink TEXT NOT NULL,
		var5qi INTEGER NOT NULL,
		arp INTEGER NOT NULL,

		dataNetworkID INTEGER NOT NULL,

		FOREIGN KEY (dataNetworkID) REFERENCES data_networks (id) ON DELETE CASCADE
)`

const (
	listPoliciesPagedStmt = "SELECT &Policy.*, COUNT(*) OVER() AS &NumItems.count from %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getPolicyStmt         = "SELECT &Policy.* from %s WHERE name==$Policy.name"
	getPolicyByIDStmt     = "SELECT &Policy.* FROM %s WHERE id==$Policy.id"
	createPolicyStmt      = "INSERT INTO %s (name, bitrateUplink, bitrateDownlink, var5qi, arp, dataNetworkID) VALUES ($Policy.name, $Policy.bitrateUplink, $Policy.bitrateDownlink, $Policy.var5qi, $Policy.arp, $Policy.dataNetworkID)"
	editPolicyStmt        = "UPDATE %s SET bitrateUplink=$Policy.bitrateUplink, bitrateDownlink=$Policy.bitrateDownlink, var5qi=$Policy.var5qi, arp=$Policy.arp, dataNetworkID=$Policy.dataNetworkID WHERE name==$Policy.name"
	deletePolicyStmt      = "DELETE FROM %s WHERE name==$Policy.name"
	countPoliciesStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

type Policy struct {
	ID              int    `db:"id"`
	Name            string `db:"name"`
	BitrateUplink   string `db:"bitrateUplink"`
	BitrateDownlink string `db:"bitrateDownlink"`
	Var5qi          int32  `db:"var5qi"`
	Arp             int32  `db:"arp"`
	DataNetworkID   int    `db:"dataNetworkID"`
}

func (db *Database) ListPoliciesPage(ctx context.Context, page int, perPage int) ([]Policy, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var policies []Policy

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	err := db.conn.Query(ctx, db.listPoliciesStmt, args).GetAll(&policies, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountPolicies(ctx)
			if countErr != nil {
				return nil, 0, nil
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

	return policies, count, nil
}

func (db *Database) GetPolicy(ctx context.Context, name string) (*Policy, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{Name: name}

	err := db.conn.Query(ctx, db.getPolicyStmt, row).Get(&row)
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

func (db *Database) GetPolicyByID(ctx context.Context, id int) (*Policy, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{ID: id}

	err := db.conn.Query(ctx, db.getPolicyByIDStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.RecordError(err)
			span.SetStatus(codes.Error, "not found")

			return nil, fmt.Errorf("policy with ID %d not found", id)
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) CreatePolicy(ctx context.Context, policy *Policy) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.createPolicyStmt, policy).Run()
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

func (db *Database) UpdatePolicy(ctx context.Context, policy *Policy) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "update").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.editPolicyStmt, policy).Get(&outcome)
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

func (db *Database) DeletePolicy(ctx context.Context, name string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "delete").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deletePolicyStmt, Policy{Name: name}).Get(&outcome)
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

// CountPolicies returns policy count
func (db *Database) CountPolicies(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countPoliciesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
