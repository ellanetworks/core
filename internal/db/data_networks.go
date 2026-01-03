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

const DataNetworksTableName = "data_networks"

const QueryCreateDataNetworksTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL UNIQUE,

		ipPool TEXT NOT NULL,
		dns TEXT NOT NULL,
		mtu INTEGER NOT NULL
)`

const (
	listDataNetworksPagedStmt = "SELECT &DataNetwork.* from %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getDataNetworkStmt        = "SELECT &DataNetwork.* from %s WHERE name==$DataNetwork.name"
	getDataNetworkByIDStmt    = "SELECT &DataNetwork.* FROM %s WHERE id==$DataNetwork.id"
	createDataNetworkStmt     = "INSERT INTO %s (name, ipPool, dns, mtu) VALUES ($DataNetwork.name, $DataNetwork.ipPool, $DataNetwork.dns, $DataNetwork.mtu)"
	editDataNetworkStmt       = "UPDATE %s SET ipPool=$DataNetwork.ipPool, dns=$DataNetwork.dns, mtu=$DataNetwork.mtu WHERE name==$DataNetwork.name"
	deleteDataNetworkStmt     = "DELETE FROM %s WHERE name==$DataNetwork.name"
	countDataNetworksStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

type DataNetwork struct {
	ID     int    `db:"id"`
	Name   string `db:"name"`
	IPPool string `db:"ipPool"`
	DNS    string `db:"dns"`
	MTU    int32  `db:"mtu"`
}

func (db *Database) ListDataNetworksPage(ctx context.Context, page, perPage int) ([]DataNetwork, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", DataNetworksTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", DataNetworksTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	count, err := db.CountDataNetworks(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "count failed")

		return nil, 0, err
	}

	var dataNetworks []DataNetwork

	err = db.conn.Query(ctx, db.listDataNetworksStmt, args).GetAll(&dataNetworks)
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

	return dataNetworks, count, nil
}

func (db *Database) GetDataNetwork(ctx context.Context, name string) (*DataNetwork, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", DataNetworksTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", DataNetworksTableName),
		),
	)
	defer span.End()

	row := DataNetwork{Name: name}

	err := db.conn.Query(ctx, db.getDataNetworkStmt, row).Get(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			span.RecordError(err)
			span.SetStatus(codes.Error, "not found")

			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, err
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) GetDataNetworkByID(ctx context.Context, id int) (*DataNetwork, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", DataNetworksTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", DataNetworksTableName),
		),
	)
	defer span.End()

	row := DataNetwork{ID: id}

	err := db.conn.Query(ctx, db.getDataNetworkByIDStmt, row).Get(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			span.RecordError(err)
			span.SetStatus(codes.Error, "not found")

			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, err
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) CreateDataNetwork(ctx context.Context, dataNetwork *DataNetwork) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", DataNetworksTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", DataNetworksTableName),
		),
	)
	defer span.End()

	err := db.conn.Query(ctx, db.createDataNetworkStmt, dataNetwork).Run()
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

func (db *Database) UpdateDataNetwork(ctx context.Context, dataNetwork *DataNetwork) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", DataNetworksTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", DataNetworksTableName),
		),
	)
	defer span.End()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.editDataNetworkStmt, dataNetwork).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return err
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return err
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteDataNetwork(ctx context.Context, name string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", DataNetworksTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", DataNetworksTableName),
		),
	)
	defer span.End()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteDataNetworkStmt, DataNetwork{Name: name}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return err
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return err
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) CountDataNetworks(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", DataNetworksTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", DataNetworksTableName),
		),
	)
	defer span.End()

	var result NumItems

	err := db.conn.Query(ctx, db.countDataNetworksStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return 0, err
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
