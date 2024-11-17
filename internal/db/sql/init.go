package sql

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema/allocated_ips.sql
var allocatedIpsTableDdl string

//go:embed schema/device_groups.sql
var deviceGroupsTableDdl string

//go:embed schema/ip_pools.sql
var ipPoolsTableDdl string

//go:embed schema/network_slices.sql
var networkSlicesTableDdl string

//go:embed schema/radios.sql
var radiosTableDdl string

//go:embed schema/upfs.sql
var upfsTableDdl string

//go:embed schema/subscribers.sql
var subscribersTableDdl string

func Initialize(dbPath string) (*Queries, error) {
	database, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	if _, err := database.ExecContext(ctx, "PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	if _, err := database.ExecContext(ctx, allocatedIpsTableDdl); err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(ctx, deviceGroupsTableDdl); err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(ctx, ipPoolsTableDdl); err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(ctx, networkSlicesTableDdl); err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(ctx, radiosTableDdl); err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(ctx, upfsTableDdl); err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(ctx, subscribersTableDdl); err != nil {
		return nil, err
	}

	queries := New(database)
	return queries, nil
}
