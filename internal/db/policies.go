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
		priorityLevel INTEGER NOT NULL,

		dataNetworkID INTEGER NOT NULL,
    	FOREIGN KEY (dataNetworkID) REFERENCES data_networks (id)
)`

const (
	listPoliciesStmt  = "SELECT &Policy.* from %s"
	getPolicyStmt     = "SELECT &Policy.* from %s WHERE name==$Policy.name"
	getPolicyByIDStmt = "SELECT &Policy.* FROM %s WHERE id==$Policy.id"
	createPolicyStmt  = "INSERT INTO %s (name, bitrateUplink, bitrateDownlink, var5qi, priorityLevel, dataNetworkID) VALUES ($Policy.name, $Policy.bitrateUplink, $Policy.bitrateDownlink, $Policy.var5qi, $Policy.priorityLevel, $Policy.dataNetworkID)"
	editPolicyStmt    = "UPDATE %s SET bitrateUplink=$Policy.bitrateUplink, bitrateDownlink=$Policy.bitrateDownlink, var5qi=$Policy.var5qi, priorityLevel=$Policy.priorityLevel, dataNetworkID=$Policy.dataNetworkID WHERE name==$Policy.name"
	deletePolicyStmt  = "DELETE FROM %s WHERE name==$Policy.name"
)

type Policy struct {
	ID              int    `db:"id"`
	Name            string `db:"name"`
	BitrateUplink   string `db:"bitrateUplink"`
	BitrateDownlink string `db:"bitrateDownlink"`
	Var5qi          int32  `db:"var5qi"`
	PriorityLevel   int32  `db:"priorityLevel"`
	DataNetworkID   int    `db:"dataNetworkID"`
}

func (db *Database) ListPolicies(ctx context.Context) ([]Policy, error) {
	operation := "SELECT"
	target := PoliciesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(listPoliciesStmt, db.policiesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Policy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	var policies []Policy
	if err := db.conn.Query(ctx, q).GetAll(&policies); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return policies, nil
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
