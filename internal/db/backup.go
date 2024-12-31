package db

import (
	"fmt"
	"io"
	"os"
	"time"
)

func (db *Database) Backup() (string, error) {
	if db.conn == nil {
		return "", fmt.Errorf("database connection is not initialized")
	}

	if _, err := os.Stat(db.filepath); err != nil {
		return "", fmt.Errorf("database file not found: %v", err)
	}

	backupFileName := fmt.Sprintf("backup_%s.db", time.Now().Format("20060102_150405"))
	backupFilePath := fmt.Sprintf("./backups/%s", backupFileName)

	if err := os.MkdirAll("./backups", os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create backups directory: %v", err)
	}

	sourceFile, err := os.Open(db.filepath)
	if err != nil {
		return "", fmt.Errorf("failed to open database file: %v", err)
	}
	defer sourceFile.Close()

	backupFile, err := os.Create(backupFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %v", err)
	}
	defer backupFile.Close()

	_, err = io.Copy(backupFile, sourceFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy database file: %v", err)
	}

	return backupFilePath, nil
}
