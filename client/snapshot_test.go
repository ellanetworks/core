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

func TestCreateSnapshot_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"result":{"message":"Snapshot created"}}`),
		},
	}
	c := &client.Client{Requester: fake}

	if err := c.CreateSnapshot(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Fatalf("expected POST, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/cluster/snapshot" {
		t.Fatalf("unexpected path: %s", fake.lastOpts.Path)
	}
}

func TestCreateSnapshot_Failure(t *testing.T) {
	fake := &fakeRequester{
		err: errors.New("server error"),
	}
	c := &client.Client{Requester: fake}

	if err := c.CreateSnapshot(context.Background()); err == nil {
		t.Fatal("expected error, got none")
	}
}

func TestRestoreSnapshot_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "test.snapshot")

	if err := os.WriteFile(tmpPath, []byte("snapshot data"), 0o644); err != nil {
		t.Fatalf("failed to write snapshot file: %v", err)
	}

	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
		},
	}
	c := &client.Client{Requester: fake}

	err := c.RestoreSnapshot(context.Background(), &client.RestoreSnapshotParams{Path: tmpPath})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Fatalf("expected POST, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/cluster/snapshot/restore" {
		t.Fatalf("unexpected path: %s", fake.lastOpts.Path)
	}
}

func TestRestoreSnapshot_NilParams(t *testing.T) {
	c := &client.Client{Requester: &fakeRequester{}}

	if err := c.RestoreSnapshot(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestRestoreSnapshot_EmptyPath(t *testing.T) {
	c := &client.Client{Requester: &fakeRequester{}}

	if err := c.RestoreSnapshot(context.Background(), &client.RestoreSnapshotParams{}); err == nil {
		t.Fatal("expected error for empty path")
	}
}
