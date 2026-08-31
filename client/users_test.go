// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
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

func TestListUsers_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"email": "1234"}, {"email": "5678"}], "page": 1, "per_page": 10, "total_count": 2}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	listUsersParams := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	resp, err := clientObj.ListUsers(ctx, listUsersParams)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 users, got: %d", len(resp.Items))
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

func TestGetUser_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"email": "alice@example.com", "role_id": 3}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	user, err := clientObj.GetUser(ctx, &client.GetUserOptions{Email: "alice@example.com"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if user.Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", user.Email)
	}

	if user.RoleID != client.RoleNetworkManager {
		t.Fatalf("expected role %d, got %d", client.RoleNetworkManager, user.RoleID)
	}

	if fake.lastOpts.Method != "GET" {
		t.Fatalf("expected GET method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/users/alice@example.com" {
		t.Fatalf("expected path api/v1/users/alice@example.com, got %s", fake.lastOpts.Path)
	}
}

func TestUpdateUser_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "User updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.UpdateUser(ctx, "alice@example.com", &client.UpdateUserOptions{RoleID: client.RoleAdmin})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected PUT method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/users/alice@example.com" {
		t.Fatalf("expected path api/v1/users/alice@example.com, got %s", fake.lastOpts.Path)
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
		Name:      "whatevername",
		ExpiresAt: "",
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

func TestListMyAPITokens_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"name": "1234"}, {"name": "5678"}], "page": 1, "per_page": 10, "total_count": 2}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	param := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	resp, err := clientObj.ListMyAPITokens(ctx, param)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 tokens, got: %d", len(resp.Items))
	}
}

func TestListUserAPITokens_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"name": "agent-token"}], "page": 1, "per_page": 10, "total_count": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	param := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	resp, err := clientObj.ListUserAPITokens(ctx, "user@example.com", param)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 token, got: %d", len(resp.Items))
	}

	if resp.Items[0].Name != "agent-token" {
		t.Fatalf("expected token name 'agent-token', got: %s", resp.Items[0].Name)
	}
}

func TestCreateUserAPIToken_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"token": "ellacore_abc123_secret456"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	opts := &client.CreateAPITokenOptions{
		Name: "ci-pipeline",
	}

	resp, err := clientObj.CreateUserAPIToken(ctx, "user@example.com", opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.Token != "ellacore_abc123_secret456" {
		t.Fatalf("expected token 'ellacore_abc123_secret456', got: %s", resp.Token)
	}
}

func TestDeleteUserAPIToken_Success(t *testing.T) {
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

	err := clientObj.DeleteUserAPIToken(ctx, "user@example.com", "token-id-123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateMyPassword_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "User password updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.UpdateMyPassword(ctx, &client.UpdateMyPasswordOptions{
		CurrentPassword: "oldpass",
		Password:        "newpass",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected method PUT, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/users/me/password" {
		t.Fatalf("expected path %q, got %q", "api/v1/users/me/password", fake.lastOpts.Path)
	}
}

func TestUpdateUserPassword_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "User password updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.UpdateUserPassword(ctx, "user@example.com", &client.UpdateUserPasswordOptions{
		Password: "newpass",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected method PUT, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/users/user@example.com/password" {
		t.Fatalf("expected path %q, got %q", "api/v1/users/user@example.com/password", fake.lastOpts.Path)
	}
}

func TestGetMyUser_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"email": "admin@ellanetworks.com", "role_id": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	user, err := clientObj.GetMyUser(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if user.Email != "admin@ellanetworks.com" {
		t.Fatalf("unexpected email: %q", user.Email)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/users/me" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}
