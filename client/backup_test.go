package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreateBackup_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "ella_core.backup")

	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Backup created successfully"}`),
			Body:       io.NopCloser(strings.NewReader("backup data")),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	createBackupOpts := &client.CreateBackupParams{
		Path: tmpPath,
	}

	ctx := context.Background()

	err := clientObj.CreateBackup(ctx, createBackupOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateBackup_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid Path"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	createBackupOpts := &client.CreateBackupParams{
		Path: "invalid/path",
	}

	ctx := context.Background()

	err := clientObj.CreateBackup(ctx, createBackupOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestRestoreBackup_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "ella_core.backup")

	err := os.WriteFile(tmpPath, []byte("backup data"), 0o644)
	if err != nil {
		t.Fatalf("failed to write temp backup file: %v", err)
	}

	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Backup restored successfully"}`),
			Body:       io.NopCloser(strings.NewReader("")),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	restoreBackupOpts := &client.RestoreBackupParams{
		Path: tmpPath,
	}

	ctx := context.Background()

	err = clientObj.RestoreBackup(ctx, restoreBackupOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRestoreBackup_Failure(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "ella_core.backup")

	err := os.WriteFile(tmpPath, []byte("backup data"), 0o644)
	if err != nil {
		t.Fatalf("failed to write temp backup file: %v", err)
	}

	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Restore failed"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	restoreBackupOpts := &client.RestoreBackupParams{
		Path: tmpPath,
	}

	ctx := context.Background()

	err = clientObj.RestoreBackup(ctx, restoreBackupOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
