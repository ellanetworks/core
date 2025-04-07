package client_test

import (
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

	err := clientObj.CreateUser(createUserOpts)
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

	err := clientObj.CreateUser(createUserOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListUsers_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"imsi": "1234"}, {"imsi": "5678"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	users, err := clientObj.ListUsers()
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

	users, err := clientObj.ListUsers()
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

	err := clientObj.DeleteUser(deleteUserOpts)
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

	err := clientObj.DeleteUser(deleteUserOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
