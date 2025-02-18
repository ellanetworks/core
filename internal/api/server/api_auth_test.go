package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt"
)

type LoginParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponseResult struct {
	Token string `json:"token"`
}

type LoginResponse struct {
	Result LoginResponseResult `json:"result"`
	Error  string              `json:"error,omitempty"`
}

type LoookupTokenResponseResult struct {
	Valid bool `json:"valid"`
}

type LoookupTokenResponse struct {
	Result LoookupTokenResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

func login(url string, client *http.Client, data *LoginParams) (int, *LoginResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/auth/login", strings.NewReader(string(body)))
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
	var loginResponse LoginResponse
	if err := json.NewDecoder(res.Body).Decode(&loginResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &loginResponse, nil
}

func lookupToken(url string, client *http.Client, token string) (int, *LoookupTokenResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/auth/lookup-token", nil)
	if err != nil {
		return 0, nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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
	var lookupResponse LoookupTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&lookupResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &lookupResponse, nil
}

func TestLoginEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, jwtSecret, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("1. Create Admin user", func(t *testing.T) {
		user := &CreateUserParams{
			Email:    "my.user123@ellanetworks.com",
			Password: "password123",
			Role:     0,
		}
		statusCode, _, err := createUser(ts.URL, client, "", user)
		if err != nil {
			t.Fatalf("couldn't create admin user: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
	})

	t.Run("2. Login success", func(t *testing.T) {
		user := &LoginParams{
			Email:    "my.user123@ellanetworks.com",
			Password: "password123",
		}
		statusCode, loginResponse, err := login(ts.URL, client, user)
		if err != nil {
			t.Fatalf("couldn't login admin user: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if loginResponse.Result.Token == "" {
			t.Fatalf("expected token, got empty string")
		}
		token, err := jwt.Parse(loginResponse.Result.Token, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})
		if err != nil {
			t.Fatalf("couldn't parse token: %s", err)
		}
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			if claims["email"] != "my.user123@ellanetworks.com" {
				t.Fatalf("expected email %q, got %q", "testuser", claims["email"])
			}
		} else {
			t.Fatalf("invalid token or claims")
		}
	})

	t.Run("3. Login failure missing email", func(t *testing.T) {
		invalidUser := &LoginParams{
			Email:    "",
			Password: "Admin123",
		}
		statusCode, loginResponse, err := login(ts.URL, client, invalidUser)
		if err != nil {
			t.Fatalf("couldn't login admin user: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if loginResponse.Error != "Email is required" {
			t.Fatalf("expected error %q, got %q", "Email is required", loginResponse.Error)
		}
	})

	t.Run("4. Login failure missing password", func(t *testing.T) {
		invalidUser := &LoginParams{
			Email:    "my.user123@ellanetworks.com",
			Password: "",
		}
		statusCode, loginResponse, err := login(ts.URL, client, invalidUser)
		if err != nil {
			t.Fatalf("couldn't login admin user: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if loginResponse.Error != "Password is required" {
			t.Fatalf("expected error %q, got %q", "Password is required", loginResponse.Error)
		}
	})

	t.Run("5. Login failure invalid password", func(t *testing.T) {
		invalidUser := &LoginParams{
			Email:    "my.user123@ellanetworks.com",
			Password: "a-wrong-password",
		}
		statusCode, loginResponse, err := login(ts.URL, client, invalidUser)
		if err != nil {
			t.Fatalf("couldn't login admin user: %s", err)
		}
		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if loginResponse.Error != "The email or password is incorrect. Try again." {
			t.Fatalf("expected error %q, got %q", "The email or password is incorrect. Try again.", loginResponse.Error)
		}
	})

	t.Run("6. Login failure invalid email", func(t *testing.T) {
		invalidUser := &LoginParams{
			Email:    "not-existing-user",
			Password: "Admin123",
		}
		statusCode, loginResponse, err := login(ts.URL, client, invalidUser)
		if err != nil {
			t.Fatalf("couldn't login admin user: %s", err)
		}
		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if loginResponse.Error != "The email or password is incorrect. Try again." {
			t.Fatalf("expected error %q, got %q", "The email or password is incorrect. Try again.", loginResponse.Error)
		}
	})
}

func TestRolesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	adminToken, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	token, err := createUserAndLogin(ts.URL, adminToken, 1, client)
	if err != nil {
		t.Fatalf("couldn't create user and login: %s", err)
	}

	t.Run("1. Use ReadOnly user to create a new user - should fail", func(t *testing.T) {
		user := &CreateUserParams{
			Email:    "whatever@ellanetworks.com",
			Password: "password123",
			Role:     1,
		}
		statusCode, response, _ := createUser(ts.URL, client, token, user)
		if err != nil {
			t.Fatalf("couldn't create user: %s", err)
		}
		if statusCode != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, statusCode)
		}
		if response.Error != "Admin role required" {
			t.Fatalf("expected error %s, got %q", "Admin role required", response.Error)
		}
	})

	t.Run("2. Use Admin user to create a new user - should succeed", func(t *testing.T) {
		user := &CreateUserParams{
			Email:    "whatever@ellanetworks.com",
			Password: "password123",
			Role:     1,
		}
		statusCode, response, err := createUser(ts.URL, client, adminToken, user)
		if err != nil {
			t.Fatalf("couldn't create user: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("expected empty error, got %q", response.Error)
		}
	})

	t.Run("3. Use ReadOnly user to list subscribers - should succeed", func(t *testing.T) {
		statusCode, response, err := listSubscribers(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list subscribers: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if len(response.Result) != 0 {
			t.Fatalf("expected 0 subscriber, got %d", len(response.Result))
		}
	})

	t.Run("4. Use ReadOnly user to list users - should fail", func(t *testing.T) {
		statusCode, response, err := listUsers(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list users: %s", err)
		}
		if statusCode != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, statusCode)
		}
		if response.Error != "Admin role required" {
			t.Fatalf("expected error %q, got %q", "Admin role required", response.Error)
		}
	})
}

func TestLookupToken(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("Lookup valid token", func(t *testing.T) {
		createUserParams := &CreateUserParams{
			Email:    "my.user123@ellanetworks.com",
			Password: "password123",
		}
		statusCode, _, err := createUser(ts.URL, client, "", createUserParams)
		if err != nil {
			t.Fatalf("couldn't create admin user: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		loginParams := &LoginParams{
			Email:    "my.user123@ellanetworks.com",
			Password: "password123",
		}
		statusCode, loginResponse, err := login(ts.URL, client, loginParams)
		if err != nil {
			t.Fatalf("couldn't login user: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if loginResponse.Result.Token == "" {
			t.Fatalf("expected token, got empty string")
		}
		statusCode, response, err := lookupToken(ts.URL, client, loginResponse.Result.Token)
		if err != nil {
			t.Fatalf("couldn't lookup token: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if !response.Result.Valid {
			t.Fatalf("expected token to be valid")
		}
	})

	t.Run("Invalid token - Bad format", func(t *testing.T) {
		invalidToken := "invalid token format"
		statusCode, response, err := lookupToken(ts.URL, client, invalidToken)
		if err != nil {
			t.Fatalf("couldn't lookup token: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Valid {
			t.Fatalf("expected token to be invalid")
		}
	})

	t.Run("Invalid token - Correct format but invalid", func(t *testing.T) {
		// Create a correctly formatted but invalid token
		invalidToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

		statusCode, response, err := lookupToken(ts.URL, client, invalidToken)
		if err != nil {
			t.Fatalf("couldn't lookup token: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Valid {
			t.Fatalf("expected token to be invalid")
		}
	})

	t.Run("Invalid token - No token", func(t *testing.T) {
		statusCode, response, err := lookupToken(ts.URL, client, "")
		if err != nil {
			t.Fatalf("couldn't lookup token: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "Authorization header is required" {
			t.Fatalf("expected error %q, got %q", "Authorization header is required", response.Error)
		}
	})
}
