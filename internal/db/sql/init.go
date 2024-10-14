package sql

import (
	"context"
	"database/sql"
	_ "embed"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema/subscribers.sql
var subscribersTableDdl string

func Initialize(dbPath string) (*Queries, error) {
	database, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(context.Background(), subscribersTableDdl); err != nil {
		return nil, err
	}
	queries := New(database)
	return queries, nil
}
