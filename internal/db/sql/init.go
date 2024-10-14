package sql

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema/subscribers.sql
var subscribersTableDdl string

//go:embed schema/device_groups.sql
var deviceGroupsTableDdl string

//go:embed schema/device_group_subscribers.sql
var deviceGroupSubscribersTableDdl string

func Initialize(dbPath string) (*Queries, error) {
	database, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := database.ExecContext(context.Background(), "PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if _, err := database.ExecContext(context.Background(), subscribersTableDdl); err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(context.Background(), deviceGroupsTableDdl); err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(context.Background(), deviceGroupSubscribersTableDdl); err != nil {
		return nil, err
	}
	queries := New(database)
	return queries, nil
}
