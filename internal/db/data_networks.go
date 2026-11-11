// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const DataNetworksTableName = "data_networks"

const dataNetworkIMSSchema = 25

const (
	listDataNetworksPagedStmt = "SELECT id, name, ipPool, ipv6Pool, dns, mtu, ims, COUNT(*) OVER() from " + DataNetworksTableName + " LIMIT ? OFFSET ?"
	listAllDataNetworksStmt   = "SELECT id, name, ipPool, ipv6Pool, dns, mtu, ims FROM " + DataNetworksTableName + " ORDER BY id ASC"
	getDataNetworkStmt        = "SELECT id, name, ipPool, ipv6Pool, dns, mtu, ims from " + DataNetworksTableName + " WHERE name==?"
	getDataNetworkByIDStmt    = "SELECT id, name, ipPool, ipv6Pool, dns, mtu, ims FROM " + DataNetworksTableName + " WHERE id==?"
	createDataNetworkStmt     = "INSERT INTO " + DataNetworksTableName + " (id, name, ipPool, ipv6Pool, dns, mtu, ims) VALUES (?, ?, ?, ?, ?, ?, ?)"
	editDataNetworkStmt       = "UPDATE " + DataNetworksTableName + " SET ipPool=?, ipv6Pool=?, dns=?, mtu=?, ims=? WHERE name==?"
	deleteDataNetworkStmt     = "DELETE FROM " + DataNetworksTableName + " WHERE name==?"
	countDataNetworksStmt     = "SELECT COUNT(*) FROM " + DataNetworksTableName
)

const dataNetworkColumnsPreV25 = "id, name, ipPool, ipv6Pool, dns, mtu, iif(name = '" + models.IMSDataNetworkName + "', TRUE, FALSE)"

const (
	listDataNetworksPagedPreV25Stmt = "SELECT " + dataNetworkColumnsPreV25 + ", COUNT(*) OVER() from " + DataNetworksTableName + " LIMIT ? OFFSET ?"
	listAllDataNetworksPreV25Stmt   = "SELECT " + dataNetworkColumnsPreV25 + " FROM " + DataNetworksTableName + " ORDER BY id ASC"
	getDataNetworkPreV25Stmt        = "SELECT " + dataNetworkColumnsPreV25 + " from " + DataNetworksTableName + " WHERE name==?"
	getDataNetworkByIDPreV25Stmt    = "SELECT " + dataNetworkColumnsPreV25 + " FROM " + DataNetworksTableName + " WHERE id==?"
	createDataNetworkPreV25Stmt     = "INSERT INTO " + DataNetworksTableName + " (id, name, ipPool, ipv6Pool, dns, mtu) VALUES (?, ?, ?, ?, ?, ?)"
	editDataNetworkPreV25Stmt       = "UPDATE " + DataNetworksTableName + " SET ipPool=?, ipv6Pool=?, dns=?, mtu=? WHERE name==?"
)

type DataNetwork struct {
	ID       string // UUIDv7
	Name     string
	IPv4Pool string
	IPv6Pool string
	DNS      string
	MTU      int32
	IMS      bool
}

func (db *Database) dataNetworkSchemaCurrent(ctx context.Context, dn *DataNetwork) (bool, error) {
	if db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		return true, nil
	}

	if dn.IMS != (dn.Name == models.IMSDataNetworkName) {
		return false, ErrMigrationPending
	}

	return false, nil
}

func createDataNetworkQuery(dn *DataNetwork, current bool) (string, []any) {
	if current {
		return createDataNetworkStmt, []any{dn.ID, dn.Name, dn.IPv4Pool, dn.IPv6Pool, dn.DNS, dn.MTU, dn.IMS}
	}

	return createDataNetworkPreV25Stmt, []any{dn.ID, dn.Name, dn.IPv4Pool, dn.IPv6Pool, dn.DNS, dn.MTU}
}

func editDataNetworkQuery(dn *DataNetwork, current bool) (string, []any) {
	if current {
		return editDataNetworkStmt, []any{dn.IPv4Pool, dn.IPv6Pool, dn.DNS, dn.MTU, dn.IMS, dn.Name}
	}

	return editDataNetworkPreV25Stmt, []any{dn.IPv4Pool, dn.IPv6Pool, dn.DNS, dn.MTU, dn.Name}
}

func (db *Database) ListDataNetworksPage(ctx context.Context, page, perPage int) ([]DataNetwork, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+DataNetworksTableName+" (paged)", DataNetworksTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	query := listDataNetworksPagedStmt
	if !db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		query = listDataNetworksPagedPreV25Stmt
	}

	dataNetworks, count, err := queryPage(ctx, db.reader(), query, scanDataNetwork, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(dataNetworks) == 0 {
		fallbackCount, countErr := db.CountDataNetworks(ctx)
		if countErr != nil {
			return nil, 0, nil
		}

		return nil, fallbackCount, nil
	}

	return dataNetworks, count, nil
}

func (db *Database) ListAllDataNetworks(ctx context.Context) ([]DataNetwork, error) {
	ctx, span, timer := startDBQuery(ctx, DataNetworksTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	query := listAllDataNetworksStmt
	if !db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		query = listAllDataNetworksPreV25Stmt
	}

	dataNetworks, err := queryAll(ctx, db.reader(), query, scanDataNetwork)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return dataNetworks, nil
}

func (db *Database) GetDataNetwork(ctx context.Context, name string) (*DataNetwork, error) {
	ctx, span, timer := startDBQuery(ctx, DataNetworksTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	query := getDataNetworkStmt
	if !db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		query = getDataNetworkPreV25Stmt
	}

	row, err := scanDataNetwork(db.reader().QueryRowContext(ctx, query, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			recordSpanError(span, err)

			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) GetDataNetworkByID(ctx context.Context, id string) (*DataNetwork, error) {
	ctx, span, timer := startDBQuery(ctx, DataNetworksTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	query := getDataNetworkByIDStmt
	if !db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		query = getDataNetworkByIDPreV25Stmt
	}

	row, err := scanDataNetwork(db.reader().QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			recordSpanError(span, err)

			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateDataNetwork(ctx context.Context, dataNetwork *DataNetwork) error {
	ctx, span, timer := startDBQuery(ctx, DataNetworksTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if dataNetwork.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("generate data network id: %w", err)
		}

		dataNetwork.ID = id.String()
	}

	_, err := opCreateDataNetwork.Invoke(ctx, db, dataNetwork)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) UpdateDataNetwork(ctx context.Context, dataNetwork *DataNetwork) error {
	ctx, span, timer := startDBQuery(ctx, DataNetworksTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opUpdateDataNetwork.Invoke(ctx, db, dataNetwork)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteDataNetwork(ctx context.Context, name string) error {
	ctx, span, timer := startDBQuery(ctx, DataNetworksTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteDataNetwork.Invoke(ctx, db, &stringPayload{Value: name})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) CountDataNetworks(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, DataNetworksTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countDataNetworksStmt)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func scanDataNetwork(row scanner) (DataNetwork, error) {
	var v DataNetwork

	if err := row.Scan(&v.ID, &v.Name, &v.IPv4Pool, &v.IPv6Pool, &v.DNS, &v.MTU, &v.IMS); err != nil {
		return DataNetwork{}, err
	}

	return v, nil
}
