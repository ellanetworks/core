// Copyright 2024 Ella Networks

package db_test

import (
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

var initialOperator = db.Operator{
	Mcc:           "001",
	Mnc:           "01",
	OperatorCode:  "0123456789ABCDEF0123456789ABCDEF",
	Sst:           1,
	Sd:            1056816,
	SupportedTACs: `["001"]`,
}

func TestConnect(t *testing.T) {
	tempDir := t.TempDir()
	db, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"), initialOperator)
	if err != nil {
		t.Fatalf("Can't connect to SQLite: %s", err)
	}
	err = db.Close()
	if err != nil {
		t.Fatalf("Can't close connection: %s", err)
	}
}
