// Copyright 2024 Ella Networks

package db

import (
	"fmt"
	"io"
	"os"
)

func (db *Database) Backup(destinationFile *os.File) error {
	if db.conn == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	if _, err := os.Stat(db.filepath); err != nil {
		return fmt.Errorf("database file not found: %v", err)
	}

	sourceFile, err := os.Open(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to open database file: %v", err)
	}
	defer sourceFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy database file: %v", err)
	}

	// Ensure all writes are flushed before passing it to the API
	if err := destinationFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync backup file: %v", err)
	}

	return nil
}
