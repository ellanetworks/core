// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
)

type RestoreResponseResult struct {
	Message string `json:"message,omitempty"`
}

type RestoreResponse struct {
	Error  string                `json:"error,omitempty"`
	Result RestoreResponseResult `json:"result,omitempty"`
}

func restore(url string, client *http.Client, token string, backupFilePath string) (int, *RestoreResponse, error) {
	file, err := os.Open(backupFilePath)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			panic(closeErr)
		}
	}()

	var requestBody bytes.Buffer

	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("backup", filepath.Base(backupFilePath))
	if err != nil {
		return 0, nil, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return 0, nil, err
	}

	if err := writer.Close(); err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/restore", &requestBody)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			panic(closeErr)
		}
	}()

	var restoreResponse RestoreResponse

	err = json.NewDecoder(res.Body).Decode(&restoreResponse)
	if err != nil {
		return res.StatusCode, nil, err
	}

	return res.StatusCode, &restoreResponse, nil
}

func TestRestoreEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServerWithRaft(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	// Create a real SQLite backup file using the database's Backup method
	// (after initialization so the backup contains the admin user)
	restoreFilePath := filepath.Join(tempDir, "restore_test.db")

	backupFile, err := os.Create(restoreFilePath)
	if err != nil {
		t.Fatalf("failed to create backup file: %s", err)
	}

	if err := env.DB.Backup(context.Background(), backupFile); err != nil {
		_ = backupFile.Close()

		t.Fatalf("failed to create backup: %s", err)
	}

	_ = backupFile.Close()

	t.Run("1. Trigger restore successfully", func(t *testing.T) {
		statusCode, restoreResponse, err := restore(env.Server.URL, client, token, restoreFilePath)
		if err != nil {
			t.Fatalf("couldn't trigger restore: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, statusCode, restoreResponse.Error)
		}

		if restoreResponse.Result.Message != "Database restored successfully" {
			t.Fatalf("expected message 'Database restored successfully', got '%s'", restoreResponse.Result.Message)
		}
	})

	t.Run("2. Trigger restore without authorization", func(t *testing.T) {
		statusCode, _, err := restore(env.Server.URL, client, "", restoreFilePath)
		if err != nil {
			t.Fatalf("couldn't trigger restore: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}
	})

	t.Run("3. Trigger restore with invalid file returns 400", func(t *testing.T) {
		invalidFilePath := filepath.Join(tempDir, "invalid.db")
		if err := os.WriteFile(invalidFilePath, []byte("not a sqlite database"), 0o600); err != nil {
			t.Fatalf("failed to write invalid file: %s", err)
		}

		statusCode, restoreResponse, err := restore(env.Server.URL, client, token, invalidFilePath)
		if err != nil {
			t.Fatalf("couldn't trigger restore: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, statusCode, restoreResponse.Error)
		}
	})

	t.Run("4. Database still works after rejected restore", func(t *testing.T) {
		// After the invalid restore in test 3, the original DB should still work.
		// Verify by triggering a successful restore.
		statusCode, restoreResponse, err := restore(env.Server.URL, client, token, restoreFilePath)
		if err != nil {
			t.Fatalf("couldn't trigger restore: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, statusCode, restoreResponse.Error)
		}
	})
}

func TestRestoreEndpointRejectsBackupAheadOfCurrentState(t *testing.T) {
	ctx := context.Background()

	env, err := setupServerWithRaft(filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatalf("create test server: %v", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("initialize server: %v", err)
	}

	if _, err := env.DB.PlainDB().ExecContext(ctx,
		"UPDATE fsm_state SET lastApplied = ? WHERE id = 1", 42); err != nil {
		t.Fatalf("set backup lastApplied: %v", err)
	}

	backupPath := filepath.Join(t.TempDir(), "backup.tar.gz")

	backupFile, err := os.Create(backupPath)
	if err != nil {
		t.Fatalf("create backup file: %v", err)
	}

	if err := env.DB.Backup(ctx, backupFile); err != nil {
		_ = backupFile.Close()

		t.Fatalf("create backup: %v", err)
	}

	if err := backupFile.Close(); err != nil {
		t.Fatalf("close backup file: %v", err)
	}

	if _, err := env.DB.PlainDB().ExecContext(ctx,
		"UPDATE fsm_state SET lastApplied = ? WHERE id = 1", 41); err != nil {
		t.Fatalf("set current lastApplied: %v", err)
	}

	statusCode, response, err := restore(env.Server.URL, client, token, backupPath)
	if err != nil {
		t.Fatalf("restore request: %v", err)
	}

	if statusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", statusCode, http.StatusConflict)
	}

	if response.Error != server.RestoreBackupAheadMessage {
		t.Fatalf("error = %q, want %q", response.Error, server.RestoreBackupAheadMessage)
	}

	var currentLastApplied int64
	if err := env.DB.PlainDB().QueryRowContext(ctx,
		"SELECT lastApplied FROM fsm_state WHERE id = 1").Scan(&currentLastApplied); err != nil {
		t.Fatalf("read current lastApplied: %v", err)
	}

	if currentLastApplied != 41 {
		t.Fatalf("current lastApplied = %d, want 41 after rejected restore", currentLastApplied)
	}
}
