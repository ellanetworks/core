package db

import (
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/canonical/sqlair"
)

func (db *Database) Restore(backupFilePath string) error {
	if db.conn == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	if _, err := os.Stat(backupFilePath); err != nil {
		return fmt.Errorf("backup file not found: %v", err)
	}

	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close the database connection: %v", err)
	}

	sourceFile, err := os.Open(backupFilePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %v", err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to open destination database file: %v", err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
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
