package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

// TestLogin_Success verifies that a successful login returns the expected token.
func TestLogin_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			// The Login function expects that the raw JSON from the "result" field contains the token.
			Result: []byte(`{"token": "testtoken"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	loginOpts := &client.LoginOptions{
		Email:    "user@example.com",
		Password: "secret",
	}

	ctx := context.Background()

	err := clientObj.Login(ctx, loginOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	err = clientObj.Refresh(ctx)
	if err != nil {
		t.Fatalf("expected no error on refresh, got: %v", err)
	}

	token := clientObj.GetToken()
	if token != "testtoken" {
		t.Errorf("expected token 'testtoken', got: %s", token)
	}
}

// TestLogin_RequesterError verifies that if the Requester returns an error, Login passes it along.
func TestLogin_RequesterError(t *testing.T) {
	fake := &fakeRequester{
		response: nil,
		err:      errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	loginOpts := &client.LoginOptions{
		Email:    "user@example.com",
		Password: "secret",
	}

	ctx := context.Background()

	err := clientObj.Login(ctx, loginOpts)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if err.Error() != "requester error" {
		t.Errorf("expected 'requester error', got: %v", err)
	}
}

// TestLogin_RequestParameters verifies that Login constructs the HTTP request correctly.
func TestLogin_RequestParameters(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"token": "testtoken"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	email := "user@example.com"
	password := "secret"
	loginOpts := &client.LoginOptions{
		Email:    email,
		Password: password,
	}

	ctx := context.Background()

	err := clientObj.Login(ctx, loginOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts == nil {
		t.Fatal("expected RequestOptions to be set, but got nil")
	}

	if fake.lastOpts.Method != "POST" {
		t.Errorf("expected method POST, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/auth/login" {
		t.Errorf("expected path 'api/v1/auth/login', got: %s", fake.lastOpts.Path)
	}

	if ct, ok := fake.lastOpts.Headers["Content-Type"]; !ok || ct != "application/json" {
		t.Errorf("expected header Content-Type to be 'application/json', got: %v", fake.lastOpts.Headers)
	}

	// Read and verify the request body.
	bodyBytes, err := io.ReadAll(fake.lastOpts.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	// Reset the body reader for potential further use.
	fake.lastOpts.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var payload map[string]string
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if payload["email"] != email {
		t.Errorf("expected email %s, got: %s", email, payload["email"])
	}

	if payload["password"] != password {
		t.Errorf("expected password %s, got: %s", password, payload["password"])
	}
}
