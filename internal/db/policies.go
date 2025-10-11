// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const PoliciesTableName = "policies"

const QueryCreatePoliciesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL,

		bitrateUplink TEXT NOT NULL,
		bitrateDownlink TEXT NOT NULL,
		var5qi INTEGER NOT NULL,
		arp INTEGER NOT NULL,

		dataNetworkID INTEGER NOT NULL,
    	FOREIGN KEY (dataNetworkID) REFERENCES data_networks (id)
)`

const (
	listPoliciesPagedStmt = "SELECT &Policy.* from %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
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
	const operation = "SELECT"
	const target = PoliciesTableName
	spanName := fmt.Sprintf("%s %s (paged)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(listPoliciesPagedStmt, db.policiesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("page", page),
		attribute.Int("per_page", perPage),
	)

	stmt, err := sqlair.Prepare(stmtStr, ListArgs{}, Policy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, 0, err
	}

	count, err := db.CountPolicies(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "count failed")
		return nil, 0, err
	}

	var policies []Policy

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	if err := db.conn.Query(ctx, stmt, args).GetAll(&policies); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, count, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, 0, err
	}

	span.SetStatus(codes.Ok, "")
	return policies, count, nil
}

func (db *Database) GetPolicy(ctx context.Context, name string) (*Policy, error) {
	operation := "SELECT"
	target := PoliciesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getPolicyStmt, db.policiesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := Policy{Name: name}
	q, err := sqlair.Prepare(stmt, Policy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) GetPolicyByID(ctx context.Context, id int) (*Policy, error) {
	operation := "SELECT"
	target := PoliciesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getPolicyByIDStmt, db.policiesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := Policy{ID: id}
	q, err := sqlair.Prepare(stmt, Policy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		if err == sql.ErrNoRows {
			span.RecordError(err)
			span.SetStatus(codes.Error, "not found")
			return nil, fmt.Errorf("policy with ID %d not found", id)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) CreatePolicy(ctx context.Context, policy *Policy) error {
	operation := "INSERT"
	target := PoliciesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(createPolicyStmt, db.policiesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure unique name
	if _, err := db.GetPolicy(ctx, policy.Name); err == nil {
		dup := fmt.Errorf("policy with name %s already exists", policy.Name)
		span.RecordError(dup)
		span.SetStatus(codes.Error, "duplicate key")
		return dup
	}

	q, err := sqlair.Prepare(stmt, Policy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, policy).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) UpdatePolicy(ctx context.Context, policy *Policy) error {
	operation := "UPDATE"
	target := PoliciesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(editPolicyStmt, db.policiesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure exists
	if _, err := db.GetPolicy(ctx, policy.Name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	q, err := sqlair.Prepare(stmt, Policy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, policy).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) DeletePolicy(ctx context.Context, name string) error {
	operation := "DELETE"
	target := PoliciesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(deletePolicyStmt, db.policiesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure exists
	if _, err := db.GetPolicy(ctx, name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	q, err := sqlair.Prepare(stmt, Policy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, Policy{Name: name}).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// CountPolicies returns policy count
func (db *Database) CountPolicies(ctx context.Context) (int, error) {
	operation := "SELECT"
	target := PoliciesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(countPoliciesStmt, db.policiesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	var result NumItems
	q, err := sqlair.Prepare(stmt, NumItems{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return 0, err
	}
	if err := db.conn.Query(ctx, q).Get(&result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return result.Count, nil
}
