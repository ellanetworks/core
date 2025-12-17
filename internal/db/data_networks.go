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
	operation := "SELECT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s (paged)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("page", page),
		attribute.Int("per_page", perPage),
	)

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

	if err := db.conn.Query(ctx, db.listDataNetworksStmt, args).GetAll(&dataNetworks); err != nil {
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
	operation := "SELECT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := DataNetwork{Name: name}

	if err := db.conn.Query(ctx, db.getDataNetworkStmt, row).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) GetDataNetworkByID(ctx context.Context, id int) (*DataNetwork, error) {
	operation := "SELECT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := DataNetwork{ID: id}

	if err := db.conn.Query(ctx, db.getDataNetworkByIDStmt, row).Get(&row); err != nil {
		if err == sql.ErrNoRows {
			span.RecordError(err)
			span.SetStatus(codes.Error, "not found")
			return nil, fmt.Errorf("data network with ID %d not found", id)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) CreateDataNetwork(ctx context.Context, dataNetwork *DataNetwork) error {
	operation := "INSERT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	if err := db.conn.Query(ctx, db.createDataNetworkStmt, dataNetwork).Run(); err != nil {
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
	operation := "UPDATE"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure exists
	if _, err := db.GetDataNetwork(ctx, dataNetwork.Name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	if err := db.conn.Query(ctx, db.editDataNetworkStmt, dataNetwork).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) DeleteDataNetwork(ctx context.Context, name string) error {
	operation := "DELETE"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure exists
	if _, err := db.GetDataNetwork(ctx, name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	if err := db.conn.Query(ctx, db.deleteDataNetworkStmt, DataNetwork{Name: name}).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) CountDataNetworks(ctx context.Context) (int, error) {
	operation := "SELECT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	var result NumItems

	if err := db.conn.Query(ctx, db.countDataNetworksStmt).Get(&result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return result.Count, nil
}
