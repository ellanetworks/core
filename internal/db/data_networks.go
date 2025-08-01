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

		name TEXT NOT NULL,

		ipPool TEXT NOT NULL,
		dns TEXT NOT NULL,
		mtu INTEGER NOT NULL
)`

const (
	listDataNetworksStmt   = "SELECT &DataNetwork.* from %s"
	getDataNetworkStmt     = "SELECT &DataNetwork.* from %s WHERE name==$DataNetwork.name"
	getDataNetworkByIDStmt = "SELECT &DataNetwork.* FROM %s WHERE id==$DataNetwork.id"
	createDataNetworkStmt  = "INSERT INTO %s (name, ipPool, dns, mtu) VALUES ($DataNetwork.name, $DataNetwork.ipPool, $DataNetwork.dns, $DataNetwork.mtu)"
	editDataNetworkStmt    = "UPDATE %s SET ipPool=$DataNetwork.ipPool, dns=$DataNetwork.dns, mtu=$DataNetwork.mtu WHERE name==$DataNetwork.name"
	deleteDataNetworkStmt  = "DELETE FROM %s WHERE name==$DataNetwork.name"
	getNumDataNetworksStmt = "SELECT COUNT(*) AS &NumDataNetworks.count FROM %s"
)

type DataNetwork struct {
	ID     int    `db:"id"`
	Name   string `db:"name"`
	IPPool string `db:"ipPool"`
	DNS    string `db:"dns"`
	MTU    int32  `db:"mtu"`
}

type NumDataNetworks struct {
	Count int `db:"count"`
}

func (db *Database) ListDataNetworks(ctx context.Context) ([]DataNetwork, error) {
	operation := "SELECT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(listDataNetworksStmt, db.dataNetworksTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, DataNetwork{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	var dataNetworks []DataNetwork
	if err := db.conn.Query(ctx, q).GetAll(&dataNetworks); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return dataNetworks, nil
}

func (db *Database) GetDataNetwork(ctx context.Context, name string) (*DataNetwork, error) {
	operation := "SELECT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getDataNetworkStmt, db.dataNetworksTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := DataNetwork{Name: name}
	q, err := sqlair.Prepare(stmt, DataNetwork{})
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

func (db *Database) GetDataNetworkByID(ctx context.Context, id int) (*DataNetwork, error) {
	operation := "SELECT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getDataNetworkByIDStmt, db.dataNetworksTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := DataNetwork{ID: id}
	q, err := sqlair.Prepare(stmt, DataNetwork{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
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

	stmt := fmt.Sprintf(createDataNetworkStmt, db.dataNetworksTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure unique name
	if _, err := db.GetDataNetwork(ctx, dataNetwork.Name); err == nil {
		dup := fmt.Errorf("data network with name %s already exists", dataNetwork.Name)
		span.RecordError(dup)
		span.SetStatus(codes.Error, "duplicate key")
		return dup
	}

	q, err := sqlair.Prepare(stmt, DataNetwork{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, dataNetwork).Run(); err != nil {
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

	stmt := fmt.Sprintf(editDataNetworkStmt, db.dataNetworksTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure exists
	if _, err := db.GetDataNetwork(ctx, dataNetwork.Name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	q, err := sqlair.Prepare(stmt, DataNetwork{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, dataNetwork).Run(); err != nil {
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

	stmt := fmt.Sprintf(deleteDataNetworkStmt, db.dataNetworksTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure exists
	if _, err := db.GetDataNetwork(ctx, name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	q, err := sqlair.Prepare(stmt, DataNetwork{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, DataNetwork{Name: name}).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// NumDataNetworks returns data network count
func (db *Database) NumDataNetworks(ctx context.Context) (int, error) {
	operation := "SELECT"
	target := DataNetworksTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getNumDataNetworksStmt, db.dataNetworksTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	var result NumDataNetworks
	q, err := sqlair.Prepare(stmt, NumDataNetworks{})
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
