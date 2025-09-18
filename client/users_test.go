package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreateUser_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "User created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	createUserOpts := &client.CreateUserOptions{
		Email:    "user@example.com",
		Password: "secret",
	}

	ctx := context.Background()

	err := clientObj.CreateUser(ctx, createUserOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateUser_Failure(t *testing.T) {
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
	createUserOpts := &client.CreateUserOptions{
		Email:    "invalid-email",
		Password: "secret",
	}

	ctx := context.Background()

	err := clientObj.CreateUser(ctx, createUserOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListUsers_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"email": "1234"}, {"email": "5678"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	users, err := clientObj.ListUsers(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got: %d", len(users))
	}
}

func TestListUsers_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Internal server error"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	users, err := clientObj.ListUsers(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
	if users != nil {
		t.Fatalf("expected no users, got: %v", users)
	}
}

func TestDeleteUser_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "User deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	deleteUserOpts := &client.DeleteUserOptions{
		Email: "admin@ellanetworks.com",
	}

	ctx := context.Background()

	err := clientObj.DeleteUser(ctx, deleteUserOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteUser_Failure(t *testing.T) {
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
	deleteUserOpts := &client.DeleteUserOptions{
		Email: "invalid-email",
	}

	ctx := context.Background()

	err := clientObj.DeleteUser(ctx, deleteUserOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestCreateMyAPIToken_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"token": "my-api-token"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	createAPITokenOpts := &client.CreateAPITokenOptions{
		Name:   "whatevername",
		Expiry: "",
	}

	ctx := context.Background()

	resp, err := clientObj.CreateMyAPIToken(ctx, createAPITokenOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Token != "my-api-token" {
		t.Fatalf("expected token 'my-api-token', got: %s", resp.Token)
	}
}

func TestCreateMyAPIToken_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid token name"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	createAPITokenOpts := &client.CreateAPITokenOptions{
		Name:   "",
		Expiry: "",
	}

	ctx := context.Background()

	resp, err := clientObj.CreateMyAPIToken(ctx, createAPITokenOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}

	if resp != nil {
		t.Fatalf("expected no response, got: %v", resp)
	}
}

func TestDeleteMyAPIToken_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "API token deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteMyAPIToken(ctx, "my-api-token-id")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteMyAPIToken_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "API token not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteMyAPIToken(ctx, "non-existent-token-id")
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListMyAPITokens_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"name": "1234"}, {"name": "5678"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	tokens, err := clientObj.ListMyAPITokens(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got: %d", len(tokens))
	}
}

func TestListMyAPITokens_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Internal server error"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	tokens, err := clientObj.ListMyAPITokens(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}

	if tokens != nil {
		t.Fatalf("expected no tokens, got: %v", tokens)
	}
}
