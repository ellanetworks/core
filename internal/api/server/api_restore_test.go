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

	ts, _, testdb, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	// Create a real SQLite backup file using the database's Backup method
	restoreFilePath := filepath.Join(tempDir, "restore_test.db")

	backupFile, err := os.Create(restoreFilePath)
	if err != nil {
		t.Fatalf("failed to create backup file: %s", err)
	}

	if err := testdb.Backup(context.Background(), backupFile); err != nil {
		_ = backupFile.Close()

		t.Fatalf("failed to create backup: %s", err)
	}

	_ = backupFile.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Trigger restore successfully", func(t *testing.T) {
		statusCode, restoreResponse, err := restore(ts.URL, client, token, restoreFilePath)
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
		statusCode, _, err := restore(ts.URL, client, "", restoreFilePath)
		if err != nil {
			t.Fatalf("couldn't trigger restore: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}
	})
}
