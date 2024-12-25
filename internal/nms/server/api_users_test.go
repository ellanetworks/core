package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	Username = "gruyaume"
	Password = "password123"
)

type GetUserResponseResult struct {
	Username string `json:"username"`
}

type GetUserResponse struct {
	Result GetUserResponseResult `json:"result"`
	Error  string                `json:"error,omitempty"`
}

type CreateUserParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateUserResponseResult struct {
	Message string `json:"message"`
}

type CreateUserResponse struct {
	Result CreateUserResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type DeleteUserResponseResult struct {
	Message string `json:"message"`
}

type DeleteUserResponse struct {
	Result DeleteUserResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

func getUser(url string, client *http.Client, name string) (int, *GetUserResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/users/"+name, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var userResponse GetUserResponse
	if err := json.NewDecoder(res.Body).Decode(&userResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &userResponse, nil
}

func createUser(url string, client *http.Client, data *CreateUserParams) (int, *CreateUserResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/users", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var createResponse CreateUserResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func editUser(url string, client *http.Client, name string, data *CreateUserParams) (int, *CreateUserResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/users/"+name, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var createResponse CreateUserResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteUser(url string, client *http.Client, name string) (int, *DeleteUserResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/users/"+name, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var deleteResponse DeleteUserResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &deleteResponse, nil
}

// This is an end-to-end test for the users handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestAPIUsersEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("1. Create user", func(t *testing.T) {
		createUserParams := &CreateUserParams{
			Username: Username,
			Password: Password,
		}
		statusCode, response, err := createUser(ts.URL, client, createUserParams)
		if err != nil {
			t.Fatalf("couldn't create user: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "User created successfully" {
			t.Fatalf("expected message %q, got %q", "User created successfully", response.Result.Message)
		}
	})

	t.Run("2. Get user", func(t *testing.T) {
		statusCode, response, err := getUser(ts.URL, client, Username)
		if err != nil {
			t.Fatalf("couldn't get user: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Username != Username {
			t.Fatalf("expected username %s, got %s", Username, response.Result.Username)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("3. Get user - id not found", func(t *testing.T) {
		statusCode, response, err := getUser(ts.URL, client, "gruyaume2")
		if err != nil {
			t.Fatalf("couldn't get user: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "User not found" {
			t.Fatalf("expected error %q, got %q", "User not found", response.Error)
		}
	})

	t.Run("4. Create user - no username", func(t *testing.T) {
		createUserParams := &CreateUserParams{
			Password: Password,
		}
		statusCode, response, err := createUser(ts.URL, client, createUserParams)
		if err != nil {
			t.Fatalf("couldn't create user: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "username is missing" {
			t.Fatalf("expected error %q, got %q", "username is missing", response.Error)
		}
	})

	t.Run("5. Edit user", func(t *testing.T) {
		createUserParams := &CreateUserParams{
			Username: Username,
			Password: "password1234",
		}
		statusCode, response, err := editUser(ts.URL, client, Username, createUserParams)
		if err != nil {
			t.Fatalf("couldn't edit user: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "User updated successfully" {
			t.Fatalf("expected message %q, got %q", "User updated successfully", response.Result.Message)
		}
	})

	t.Run("6. Delete user - success", func(t *testing.T) {
		statusCode, response, err := deleteUser(ts.URL, client, Username)
		if err != nil {
			t.Fatalf("couldn't delete user: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "User deleted successfully" {
			t.Fatalf("expected message %q, got %q", "User deleted successfully", response.Result.Message)
		}
	})
	t.Run("7. Delete user - no user", func(t *testing.T) {
		statusCode, response, err := deleteUser(ts.URL, client, Username)
		if err != nil {
			t.Fatalf("couldn't delete user: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "User not found" {
			t.Fatalf("expected error %q, got %q", "User not found", response.Error)
		}
	})
}

func TestCreateUserInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	tests := []struct {
		username string
		password string
		error    string
	}{
		{
			username: strings.Repeat("a", 257),
			password: Password,
			error:    "Invalid username format. Must be less than 256 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			createUserParams := &CreateUserParams{
				Username: tt.username,
				Password: tt.password,
			}
			statusCode, response, err := createUser(ts.URL, client, createUserParams)
			if err != nil {
				t.Fatalf("couldn't create user: %s", err)
			}
			if statusCode != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
			}
			if response.Error != tt.error {
				t.Fatalf("expected error %q, got %q", tt.error, response.Error)
			}
		})
	}
}
