package client_test

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGenerateSupportBundle_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "ella_support.tar.gz")

	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Support bundle generated"}`),
			Body:       io.NopCloser(strings.NewReader("bundle data")),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	params := &client.GenerateSupportBundleParams{Path: tmpPath}

	ctx := context.Background()

	err := clientObj.GenerateSupportBundle(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestGenerateSupportBundle_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "generation failed"}`),
		},
		err: io.ErrUnexpectedEOF,
	}
	clientObj := &client.Client{Requester: fake}

	params := &client.GenerateSupportBundleParams{Path: ""}
	ctx := context.Background()

	// Empty path should cause immediate validation error
	err := clientObj.GenerateSupportBundle(ctx, params)
	if err == nil {
		t.Fatalf("expected error for empty path, got none")
	}

	// Provide path but requester returns error
	params.Path = filepath.Join(t.TempDir(), "ella_support.tar.gz")

	err = clientObj.GenerateSupportBundle(ctx, params)
	if err == nil {
		t.Fatalf("expected error from requester, got none")
	}
}
