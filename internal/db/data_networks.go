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
	listDataNetworksPagedSQL = "SELECT id, name, ipPool, ipv6Pool, dns, mtu, ims, COUNT(*) OVER() from " + DataNetworksTableName + " LIMIT ? OFFSET ?"
	listAllDataNetworksSQL   = "SELECT id, name, ipPool, ipv6Pool, dns, mtu, ims FROM " + DataNetworksTableName + " ORDER BY id ASC"
	getDataNetworkSQL        = "SELECT id, name, ipPool, ipv6Pool, dns, mtu, ims from " + DataNetworksTableName + " WHERE name==?"
	getDataNetworkByIDSQL    = "SELECT id, name, ipPool, ipv6Pool, dns, mtu, ims FROM " + DataNetworksTableName + " WHERE id==?"
	createDataNetworkSQL     = "INSERT INTO " + DataNetworksTableName + " (id, name, ipPool, ipv6Pool, dns, mtu, ims) VALUES (?, ?, ?, ?, ?, ?, ?)"
	editDataNetworkSQL       = "UPDATE " + DataNetworksTableName + " SET ipPool=?, ipv6Pool=?, dns=?, mtu=?, ims=? WHERE name==?"
	deleteDataNetworkSQL     = "DELETE FROM " + DataNetworksTableName + " WHERE name==?"
	countDataNetworksSQL     = "SELECT COUNT(*) FROM " + DataNetworksTableName
)

const dataNetworkColumnsPreV25 = "id, name, ipPool, ipv6Pool, dns, mtu, iif(name = '" + models.IMSDataNetworkName + "', TRUE, FALSE)"

const (
	listDataNetworksPagedPreV25SQL = "SELECT " + dataNetworkColumnsPreV25 + ", COUNT(*) OVER() from " + DataNetworksTableName + " LIMIT ? OFFSET ?"
	listAllDataNetworksPreV25SQL   = "SELECT " + dataNetworkColumnsPreV25 + " FROM " + DataNetworksTableName + " ORDER BY id ASC"
	getDataNetworkPreV25SQL        = "SELECT " + dataNetworkColumnsPreV25 + " from " + DataNetworksTableName + " WHERE name==?"
	getDataNetworkByIDPreV25SQL    = "SELECT " + dataNetworkColumnsPreV25 + " FROM " + DataNetworksTableName + " WHERE id==?"
	createDataNetworkPreV25SQL     = "INSERT INTO " + DataNetworksTableName + " (id, name, ipPool, ipv6Pool, dns, mtu) VALUES (?, ?, ?, ?, ?, ?)"
	editDataNetworkPreV25SQL       = "UPDATE " + DataNetworksTableName + " SET ipPool=?, ipv6Pool=?, dns=?, mtu=? WHERE name==?"
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
		return createDataNetworkSQL, []any{dn.ID, dn.Name, dn.IPv4Pool, dn.IPv6Pool, dn.DNS, dn.MTU, dn.IMS}
	}

	return createDataNetworkPreV25SQL, []any{dn.ID, dn.Name, dn.IPv4Pool, dn.IPv6Pool, dn.DNS, dn.MTU}
}

func editDataNetworkQuery(dn *DataNetwork, current bool) (string, []any) {
	if current {
		return editDataNetworkSQL, []any{dn.IPv4Pool, dn.IPv6Pool, dn.DNS, dn.MTU, dn.IMS, dn.Name}
	}

	return editDataNetworkPreV25SQL, []any{dn.IPv4Pool, dn.IPv6Pool, dn.DNS, dn.MTU, dn.Name}
}

func (db *Database) ListDataNetworksPage(ctx context.Context, page, perPage int) (_ []DataNetwork, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+DataNetworksTableName+" (paged)", DataNetworksTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	query := listDataNetworksPagedSQL
	if !db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		query = listDataNetworksPagedPreV25SQL
	}

	dataNetworks, count, err := queryPageWithTotal(ctx, db.reader(), query, scanDataNetwork, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(dataNetworks) == 0 {
		fallbackCount, err := db.CountDataNetworks(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	return dataNetworks, count, nil
}

func (db *Database) ListAllDataNetworks(ctx context.Context) (_ []DataNetwork, err error) {
	ctx, finish := startDBQuery(ctx, DataNetworksTableName, "SELECT")
	defer func() { finish(err) }()

	query := listAllDataNetworksSQL
	if !db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		query = listAllDataNetworksPreV25SQL
	}

	dataNetworks, err := queryAll(ctx, db.reader(), query, scanDataNetwork)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return dataNetworks, nil
}

func (db *Database) GetDataNetwork(ctx context.Context, name string) (_ *DataNetwork, err error) {
	ctx, finish := startDBQuery(ctx, DataNetworksTableName, "SELECT")
	defer func() { finish(err) }()

	query := getDataNetworkSQL
	if !db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		query = getDataNetworkPreV25SQL
	}

	row, err := scanDataNetwork(db.reader().QueryRowContext(ctx, query, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) GetDataNetworkByID(ctx context.Context, id string) (_ *DataNetwork, err error) {
	ctx, finish := startDBQuery(ctx, DataNetworksTableName, "SELECT")
	defer func() { finish(err) }()

	query := getDataNetworkByIDSQL
	if !db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		query = getDataNetworkByIDPreV25SQL
	}

	row, err := scanDataNetwork(db.reader().QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateDataNetwork(ctx context.Context, dataNetwork *DataNetwork) (err error) {
	ctx, finish := startDBQuery(ctx, DataNetworksTableName, "INSERT")
	defer func() { finish(err) }()

	if dataNetwork.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate data network id: %w", err)
		}

		dataNetwork.ID = id.String()
	}

	_, err = opCreateDataNetwork.Invoke(ctx, db, dataNetwork)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) UpdateDataNetwork(ctx context.Context, dataNetwork *DataNetwork) (err error) {
	ctx, finish := startDBQuery(ctx, DataNetworksTableName, "UPDATE")
	defer func() { finish(err) }()

	_, err = opUpdateDataNetwork.Invoke(ctx, db, dataNetwork)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteDataNetwork(ctx context.Context, name string) (err error) {
	ctx, finish := startDBQuery(ctx, DataNetworksTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteDataNetwork.Invoke(ctx, db, &stringPayload{Value: name})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) CountDataNetworks(ctx context.Context) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, DataNetworksTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countDataNetworksSQL)
	if err != nil {
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
