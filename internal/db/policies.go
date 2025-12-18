// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

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

	err = db.conn.Query(ctx, db.listPoliciesStmt, args).GetAll(&policies)
	if err != nil {
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

	row := Policy{Name: name}

	err := db.conn.Query(ctx, db.getPolicyStmt, row).Get(&row)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
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

	row := Policy{ID: id}

	err := db.conn.Query(ctx, db.getPolicyByIDStmt, row).Get(&row)
	if err != nil {
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

	err := db.conn.Query(ctx, db.createPolicyStmt, policy).Run()
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")
			return ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
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

	_, err := db.GetPolicy(ctx, policy.Name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	err = db.conn.Query(ctx, db.editPolicyStmt, policy).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
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

	_, err := db.GetPolicy(ctx, name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	err = db.conn.Query(ctx, db.deletePolicyStmt, Policy{Name: name}).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
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

	var result NumItems

	err := db.conn.Query(ctx, db.countPoliciesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
