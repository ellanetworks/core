// Copyright 2024 Ella Networks

package db

import (
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func (db *Database) Restore(backupFile *os.File) error {
	if db.conn == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	if backupFile == nil {
		return fmt.Errorf("backup file is nil")
	}

	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close the database connection: %v", err)
	}

	destinationFile, err := os.Create(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to open destination database file: %v", err)
	}
	defer func() {
		if err := destinationFile.Close(); err != nil {
			logger.DBLog.Error("Failed to close destination database file", zap.Error(err))
		}
	}()

	_, err = io.Copy(destinationFile, backupFile)
	if err != nil {
		return fmt.Errorf("failed to restore database file: %v", err)
	}

	sqlConnection, err := sql.Open("sqlite3", db.filepath)
	if err != nil {
		return fmt.Errorf("failed to reopen database connection: %v", err)
	}
	db.conn = sqlair.NewDB(sqlConnection)

	return nil
}
