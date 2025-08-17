package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	Email    = "gruyaume@ellanetworks.com"
	Password = "password123"
)

type RoleID int

const (
	RoleAdmin          RoleID = 1
	RoleReadOnly       RoleID = 2
	RoleNetworkManager RoleID = 3
)

type ListUsersResponse struct {
	Result []GetUserResponseResult `json:"result"`
	Error  string                  `json:"error,omitempty"`
}

type GetUserResponseResult struct {
	Email  string `json:"email"`
	RoleID RoleID `json:"role_id"`
}

type GetUserResponse struct {
	Result GetUserResponseResult `json:"result"`
	Error  string                `json:"error,omitempty"`
}

type CreateUserParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleID   RoleID `json:"role_id"`
}

type UpdateUserPasswordParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateMyUserPasswordParams struct {
	Password string `json:"password"`
}

type UpdateUserParams struct {
	Email  string `json:"email"`
	RoleID RoleID `json:"role_id"`
}

type CreateUserResponseResult struct {
	Message string `json:"message"`
}

type CreateUserResponse struct {
	Result CreateUserResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type UpdateUserPasswordResponseResult struct {
	Message string `json:"message"`
}

type UpdateUserPasswordResponse struct {
	Result UpdateUserPasswordResponseResult `json:"result"`
	Error  string                           `json:"error,omitempty"`
}

type UpdateUserResponseResult struct {
	Message string `json:"message"`
}

type UpdateUserResponse struct {
	Result UpdateUserResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type DeleteUserResponseResult struct {
	Message string `json:"message"`
}

type DeleteUserResponse struct {
	Result DeleteUserResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type CreateAPITokenParams struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

type CreateAPITokenResponseResult struct {
	Token string `json:"token"`
}

type CreateAPITokenResponse struct {
	Result CreateAPITokenResponseResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

func listUsers(url string, client *http.Client, token string) (int, *ListUsersResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/users", nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var userResponse ListUsersResponse
	if err := json.NewDecoder(res.Body).Decode(&userResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &userResponse, nil
}

func getUser(url string, client *http.Client, token string, name string) (int, *GetUserResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/users/"+name, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
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

func createUser(url string, client *http.Client, token string, data *CreateUserParams) (int, *CreateUserResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/users", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return res.StatusCode, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var createResponse CreateUserResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return res.StatusCode, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func editUserPassword(url string, client *http.Client, token string, name string, data *UpdateUserPasswordParams) (int, *UpdateUserPasswordResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/users/"+name+"/password", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var updateResponse UpdateUserPasswordResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func editMyUserPassword(url string, client *http.Client, token string, data *UpdateMyUserPasswordParams) (int, *UpdateUserPasswordResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/users/me/password", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var updateResponse UpdateUserPasswordResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func editUser(url string, client *http.Client, token string, name string, data *UpdateUserParams) (int, *UpdateUserResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/users/"+name, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var updateResponse UpdateUserResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func deleteUser(url string, client *http.Client, token string, name string) (int, *DeleteUserResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/users/"+name, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
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

func createAPIToken(url string, client *http.Client, token string, data *CreateAPITokenParams) (int, *CreateAPITokenResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/users/me/api-tokens", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var createResponse CreateAPITokenResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteAPIToken(url string, client *http.Client, token string, tokenID string) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/users/me/api-tokens/"+tokenID, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, fmt.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
	return res.StatusCode, nil
}

// This is an end-to-end test for the users handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestAPIUsersEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Create admin user", func(t *testing.T) {
		createUserParams := &CreateUserParams{
			Email:    Email,
			Password: Password,
			RoleID:   RoleAdmin,
		}
		statusCode, response, err := createUser(ts.URL, client, token, createUserParams)
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
		statusCode, response, err := getUser(ts.URL, client, token, Email)
		if err != nil {
			t.Fatalf("couldn't get user: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Email != Email {
			t.Fatalf("expected email %s, got %s", Email, response.Result.Email)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("3. Get user - email not found", func(t *testing.T) {
		statusCode, response, err := getUser(ts.URL, client, token, "gruyaume2@ellanetworks.com")
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

	t.Run("4. Create admin user - no email", func(t *testing.T) {
		createUserParams := &CreateUserParams{
			Password: Password,
			RoleID:   RoleAdmin,
		}
		statusCode, response, err := createUser(ts.URL, client, token, createUserParams)
		if err != nil {
			t.Fatalf("couldn't create user: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "email is missing" {
			t.Fatalf("expected error %q, got %q", "email is missing", response.Error)
		}
	})

	t.Run("5. Edit user password", func(t *testing.T) {
		updateUserPasswordParams := &UpdateUserPasswordParams{
			Email:    Email,
			Password: "password1234",
		}
		statusCode, response, err := editUserPassword(ts.URL, client, token, Email, updateUserPasswordParams)
		if err != nil {
			t.Fatalf("couldn't edit user: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "User password updated successfully" {
			t.Fatalf("expected message %q, got %q", "User password updated successfully", response.Result.Message)
		}
	})

	t.Run("6. Edit user", func(t *testing.T) {
		updateUserParams := &UpdateUserParams{
			Email:  Email,
			RoleID: RoleReadOnly,
		}
		statusCode, response, err := editUser(ts.URL, client, token, Email, updateUserParams)
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

	t.Run("7. Get user", func(t *testing.T) {
		statusCode, response, err := getUser(ts.URL, client, token, Email)
		if err != nil {
			t.Fatalf("couldn't get user: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Email != Email {
			t.Fatalf("expected email %s, got %s", Email, response.Result.Email)
		}
		if response.Result.RoleID != RoleReadOnly {
			t.Fatalf("expected role %v, got %v", RoleReadOnly, response.Result.RoleID)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("8. Delete user - success", func(t *testing.T) {
		statusCode, response, err := deleteUser(ts.URL, client, token, Email)
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
	t.Run("9. Delete user - no user", func(t *testing.T) {
		statusCode, response, err := deleteUser(ts.URL, client, token, Email)
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

func TestNonAdminUpdateUserPassword(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	adminToken, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	createUserParams := &CreateUserParams{
		Email:    "ro@ellanetworks.com",
		Password: Password,
		RoleID:   RoleReadOnly,
	}

	statusCode, response, err := createUser(ts.URL, client, adminToken, createUserParams)
	if err != nil {
		t.Fatalf("couldn't create user: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	if response.Error != "" {
		t.Fatalf("unexpected error :%q", response.Error)
	}

	loginParams := &LoginParams{
		Email:    "ro@ellanetworks.com",
		Password: Password,
	}

	statusCode, loginResp, err := login(ts.URL, client, loginParams)
	if err != nil {
		t.Fatalf("couldn't login as read-only user: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if loginResp.Error != "" {
		t.Fatalf("unexpected error during login: %q", loginResp.Error)
	}

	roToken := loginResp.Result.Token

	updateUserPasswordParams := &UpdateMyUserPasswordParams{
		Password: "newpassword123",
	}

	statusCode, updateResponse, err := editMyUserPassword(ts.URL, client, roToken, updateUserPasswordParams)
	if err != nil {
		t.Fatalf("couldn't edit user password: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if updateResponse.Error != "" {
		t.Fatalf("unexpected error :%q", updateResponse.Error)
	}

	if updateResponse.Result.Message != "User password updated successfully" {
		t.Fatalf("expected message %q, got %q", "User password updated successfully", updateResponse.Result.Message)
	}
}

func TestAPIUsersFirstUserNonAdmin(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	createUserParams := &CreateUserParams{
		Email:    Email,
		Password: Password,
		RoleID:   RoleReadOnly,
	}

	statusCode, response, err := createUser(ts.URL, client, "", createUserParams)
	if err != nil {
		t.Fatalf("couldn't create user: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
	}

	if response.Error != "First user must be an admin" {
		t.Fatalf("unexpected error :%q", response.Error)
	}
}

func TestCreateUserInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		email    string
		password string
		error    string
	}{
		{
			email:    strings.Repeat("a", 257),
			password: Password,
			error:    "Invalid email format",
		},
		{
			email:    "abcdef",
			password: Password,
			error:    "Invalid email format",
		},
		{
			email:    "abcdef@",
			password: Password,
			error:    "Invalid email format",
		},
		{
			email:    "abcdef@gmail.",
			password: Password,
			error:    "Invalid email format",
		},
		{
			email:    "abcd@ef@ellanetworks.com",
			password: Password,
			error:    "Invalid email format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			createUserParams := &CreateUserParams{
				Email:    tt.email,
				Password: tt.password,
			}
			statusCode, response, err := createUser(ts.URL, client, token, createUserParams)
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

func TestCreateTooManyUsers(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	for i := 0; i < 49; i++ { // We use 49 instead of 50 because the first user was just created
		createUserParams := &CreateUserParams{
			Email:    "user" + strconv.Itoa(i) + "@ellanetworks.com",
			Password: Password,
			RoleID:   RoleReadOnly,
		}
		statusCode, response, err := createUser(ts.URL, client, token, createUserParams)
		if err != nil {
			t.Fatalf("couldn't create user: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	}

	createUserParams := &CreateUserParams{
		Email:    "abc@ellanetworks.com",
		Password: Password,
		RoleID:   RoleReadOnly,
	}
	statusCode, response, err := createUser(ts.URL, client, token, createUserParams)
	if err != nil {
		t.Fatalf("couldn't create user: %s", err)
	}
	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
	}
	if response.Error != "Maximum number of users reached (50)" {
		t.Fatalf("expected error %q, got %q", "Maximum number of users reached (50)", response.Error)
	}
}
