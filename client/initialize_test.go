package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestInitialize_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "System initialized successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	initializeOpts := &client.InitializeOptions{
		Email:    "user@example.com",
		Password: "secret",
	}

	ctx := context.Background()

	err := clientObj.Initialize(ctx, initializeOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestInitialize_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid email"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	initializeOpts := &client.InitializeOptions{
		Email:    "invalid-email",
		Password: "secret",
	}

	ctx := context.Background()

	err := clientObj.Initialize(ctx, initializeOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
