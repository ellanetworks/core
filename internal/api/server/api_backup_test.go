package server_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func backup(url string, client *http.Client, token string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/backup", nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, err
	}
	return res.StatusCode, body, nil
}

func TestBackupEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()
	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Trigger backup successfully", func(t *testing.T) {
		statusCode, body, err := backup(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't trigger backup: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		// Check that the backup file was created and returned
		backupFilePath := filepath.Join(tempDir, "backup_test.db")
		err = os.WriteFile(backupFilePath, body, 0o644)
		if err != nil {
			t.Fatalf("couldn't write backup file: %s", err)
		}

		// Verify the backup file size matches the original database size
		originalFileInfo, err := os.Stat(dbPath)
		if err != nil {
			t.Fatalf("couldn't stat original database file: %s", err)
		}
		backupFileInfo, err := os.Stat(backupFilePath)
		if err != nil {
			t.Fatalf("couldn't stat backup file: %s", err)
		}
		if originalFileInfo.Size() != backupFileInfo.Size() {
			t.Fatalf("backup file size mismatch: expected %d, got %d", originalFileInfo.Size(), backupFileInfo.Size())
		}

		// Cleanup: Delete the test backup file
		err = os.Remove(backupFilePath)
		if err != nil {
			t.Fatalf("couldn't delete backup file: %s", err)
		}
	})

	t.Run("2. Trigger backup without authorization", func(t *testing.T) {
		statusCode, _, err := backup(ts.URL, client, "")
		if err != nil {
			t.Fatalf("couldn't trigger backup: %s", err)
		}
		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}
	})
}
