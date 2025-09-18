package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetStatus_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"version": "v0.0.1", "initialized": true}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	status, err := clientObj.GetStatus(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if status.Version != "v0.0.1" {
		t.Fatalf("expected ID %v, got %v", "v0.0.1", status.Version)
	}

	if status.Initialized != true {
		t.Fatalf("expected initialized %v, got %v", true, status.Initialized)
	}
}

func TestGetStatus_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Status not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetStatus(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
