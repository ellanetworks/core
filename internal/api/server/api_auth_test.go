package server_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/golang-jwt/jwt/v5"
)

type LoginParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponseResult struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

type LoginResponse struct {
	Result LoginResponseResult `json:"result"`
	Error  string              `json:"error,omitempty"`
}

type RefreshResponseResult struct {
	Token string `json:"token"`
}

type RefreshResponse struct {
	Result RefreshResponseResult `json:"result"`
	Error  string                `json:"error,omitempty"`
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

func refresh(url string, client *http.Client) (int, *RefreshResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/auth/refresh", nil)
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

	var refreshResponse RefreshResponse
	if err := json.NewDecoder(res.Body).Decode(&refreshResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &refreshResponse, nil
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

func logout(url string, client *http.Client) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/auth/logout", nil)
	if err != nil {
		return 0, err
	}

	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	return res.StatusCode, nil
}

func TestLoginEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, jwtSecret, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	t.Run("1. Initialize", func(t *testing.T) {
		initParams := &InitializeParams{
			Email:    FirstUserEmail,
			Password: "password123",
		}

		statusCode, _, err := initialize(ts.URL, client, initParams)
		if err != nil {
			t.Fatalf("couldn't create admin user: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
	})

	t.Run("2. Login success", func(t *testing.T) {
		user := &LoginParams{
			Email:    FirstUserEmail,
			Password: "password123",
		}

		statusCode, _, err := login(ts.URL, client, user)
		if err != nil {
			t.Fatalf("couldn't login admin user: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		statusCode, refreshResponse, err := refresh(ts.URL, client)
		if err != nil {
			t.Fatalf("couldn't refresh: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if refreshResponse.Result.Token == "" {
			t.Fatalf("expected token, got empty string")
		}

		token, err := jwt.Parse(refreshResponse.Result.Token, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}

			return jwtSecret, nil
		})
		if err != nil {
			t.Fatalf("couldn't parse token: %s", err)
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			if claims["email"] != FirstUserEmail {
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
			Email:    FirstUserEmail,
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
			Email:    FirstUserEmail,
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

func TestAuthAPITokenEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	adminToken, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	var (
		APIToken   string
		APITokenID string
	)

	t.Run("1. Create API token", func(t *testing.T) {
		createAPITokenParams := &CreateAPITokenParams{
			Name:      "TestAPIToken",
			ExpiresAt: "",
		}

		statusCode, response, err := createAPIToken(ts.URL, client, adminToken, createAPITokenParams)
		if err != nil {
			t.Fatalf("couldn't create API token: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected empty error, got %q", response.Error)
		}

		if response.Result.Token == "" {
			t.Fatalf("expected token, got empty string")
		}

		tokenParts := strings.Split(response.Result.Token, "_")
		if len(tokenParts) < 3 {
			t.Fatalf("expected token format 'ellacore_<id>_<secret>', got %q", APIToken)
		}

		if tokenParts[0] != "ellacore" {
			t.Fatalf("expected token prefix 'ellacore', got %q", tokenParts[0])
		}

		APIToken = response.Result.Token
		APITokenID = tokenParts[1]
	})

	t.Run("2. Perform API request with token", func(t *testing.T) {
		statusCode, response, err := listUsers(ts.URL, client, APIToken, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list users: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 1 {
			t.Fatalf("expected 1 user, got %d", len(response.Result.Items))
		}
	})

	t.Run("3. Try to perform API request with invalid token - should fail", func(t *testing.T) {
		invalidToken := APIToken[:len(APIToken)-3] + "xyz"

		statusCode, response, err := listUsers(ts.URL, client, invalidToken, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list users with invalid token: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if response.Error != "Invalid token" {
			t.Fatalf("expected error %q, got %q", "Invalid token", response.Error)
		}
	})

	t.Run("4. Delete API token", func(t *testing.T) {
		statusCode, err := deleteAPIToken(ts.URL, client, adminToken, APITokenID)
		if err != nil {
			t.Fatalf("couldn't delete API token: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})

	t.Run("5. Try to perform API request with deleted token - should fail", func(t *testing.T) {
		statusCode, response, err := listUsers(ts.URL, client, APIToken, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list users with deleted token: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if response.Error != "Invalid token" {
			t.Fatalf("expected error %q, got %q", "Invalid token", response.Error)
		}
	})
}

func TestRefreshAfterUserDeletion(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	adminToken, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Delete user", func(t *testing.T) {
		statusCode, _, err := deleteUser(ts.URL, client, adminToken, FirstUserEmail)
		if err != nil {
			t.Fatalf("couldn't list users: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})

	t.Run("2. Refresh", func(t *testing.T) {
		statusCode, resp, err := refresh(ts.URL, client)
		if err != nil {
			t.Fatalf("couldn't refresh token: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if resp.Error != "Invalid session token" {
			t.Fatalf("expected error %q, got %q", "Invalid session token", resp.Error)
		}
	})
}

func TestRolesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	adminToken, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	readOnlyToken, err := createUserAndLogin(ts.URL, adminToken, "readonly@ellanetworks.com", RoleReadOnly, client)
	if err != nil {
		t.Fatalf("couldn't create readonly user and login: %s", err)
	}

	networkManagerToken, err := createUserAndLogin(ts.URL, adminToken, "networkmanager@ellanetworks.com", RoleNetworkManager, client)
	if err != nil {
		t.Fatalf("couldn't create network manager user and login: %s", err)
	}

	t.Run("1. Use ReadOnly user to create a new user - should fail", func(t *testing.T) {
		newUser := &CreateUserParams{
			Email:    "whatever@ellanetworks.com",
			Password: "password123",
			RoleID:   RoleReadOnly,
		}

		statusCode, response, _ := createUser(ts.URL, client, readOnlyToken, newUser)
		if statusCode != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, statusCode)
		}

		if response.Error != "Forbidden" {
			t.Fatalf("expected error %s, got %q", "Forbidden", response.Error)
		}
	})

	t.Run("2. Use Network Manager user to create a new user - should fail", func(t *testing.T) {
		user := &CreateUserParams{
			Email:    "whatever@ellanetworks.com",
			Password: "password123",
			RoleID:   RoleReadOnly,
		}
		statusCode, response, _ := createUser(ts.URL, client, networkManagerToken, user)

		if err != nil {
			t.Fatalf("couldn't create user: %s", err)
		}

		if statusCode != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, statusCode)
		}

		if response.Error != "Forbidden" {
			t.Fatalf("expected error %s, got %q", "Forbidden", response.Error)
		}
	})

	t.Run("3. Use Admin user to create a new user - should succeed", func(t *testing.T) {
		user := &CreateUserParams{
			Email:    "whatever@ellanetworks.com",
			Password: "password123",
			RoleID:   RoleReadOnly,
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

	t.Run("4. Use ReadOnly user to list subscribers - should succeed", func(t *testing.T) {
		statusCode, response, err := listSubscribers(ts.URL, client, readOnlyToken, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list subscribers: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 0 {
			t.Fatalf("expected 0 subscriber, got %d", len(response.Result.Items))
		}
	})

	t.Run("5. Use Network Manager user to list subscribers - should succeed", func(t *testing.T) {
		statusCode, response, err := listSubscribers(ts.URL, client, networkManagerToken, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list subscribers: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 0 {
			t.Fatalf("expected 0 subscriber, got %d", len(response.Result.Items))
		}
	})

	t.Run("6. Use ReadOnly user to list users - should fail", func(t *testing.T) {
		statusCode, response, err := listUsers(ts.URL, client, readOnlyToken, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list users: %s", err)
		}

		if statusCode != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, statusCode)
		}

		if response.Error != "Forbidden" {
			t.Fatalf("expected error %q, got %q", "Forbidden", response.Error)
		}
	})

	t.Run("7. Use Network Manager user to list users - should fail", func(t *testing.T) {
		statusCode, response, err := listUsers(ts.URL, client, networkManagerToken, 1, 10)
		if err != nil {
			t.Fatalf("couldn't list users: %s", err)
		}

		if statusCode != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, statusCode)
		}

		if response.Error != "Forbidden" {
			t.Fatalf("expected error %q, got %q", "Forbidden", response.Error)
		}
	})

	t.Run("8. Use Network Manager user to create data network - should succeed", func(t *testing.T) {
		createDataNetworkParams := &CreateDataNetworkParams{
			Name:   DataNetworkName,
			IPPool: "1.2.3.0/24",
			MTU:    1500,
			DNS:    "3.2.2.1",
		}

		statusCode, response, err := createDataNetwork(ts.URL, client, networkManagerToken, createDataNetworkParams)
		if err != nil {
			t.Fatalf("couldn't create data network: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected empty error, got %q", response.Error)
		}
	})

	t.Run("9. Use Network Manager user to create policy - should succeed", func(t *testing.T) {
		createPolicyParams := &CreatePolicyParams{
			Name:            PolicyName,
			BitrateUplink:   "100 Mbps",
			BitrateDownlink: "200 Mbps",
			Var5qi:          9,
			Arp:             1,
			DataNetworkName: DataNetworkName,
		}

		statusCode, response, err := createPolicy(ts.URL, client, networkManagerToken, createPolicyParams)
		if err != nil {
			t.Fatalf("couldn't create policy: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected empty error, got %q", response.Error)
		}
	})
}

func TestLookupToken(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	t.Run("Lookup valid token", func(t *testing.T) {
		initializeParams := &InitializeParams{
			Email:    FirstUserEmail,
			Password: "password123",
		}

		statusCode, _, err := initialize(ts.URL, client, initializeParams)
		if err != nil {
			t.Fatalf("couldn't create admin user: %s", err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		loginParams := &LoginParams{
			Email:    FirstUserEmail,
			Password: "password123",
		}

		statusCode, _, err = login(ts.URL, client, loginParams)
		if err != nil {
			t.Fatalf("couldn't login user: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		statusCode, refreshResponse, err := refresh(ts.URL, client)
		if err != nil {
			t.Fatalf("couldn't refresh: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if refreshResponse.Result.Token == "" {
			t.Fatalf("expected non-empty token from refresh")
		}

		statusCode, response, err := lookupToken(ts.URL, client, refreshResponse.Result.Token)
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

func TestLogout(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize and login: %s", err)
	}

	t.Run("Success - logout with valid session", func(t *testing.T) {
		statusCode, err := logout(ts.URL, client)
		if err != nil {
			t.Fatalf("couldn't logout: %s", err)
		}

		if statusCode != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, statusCode)
		}

		// Verify session cookie is cleared
		cookies := client.Jar.Cookies(mustParseURL(ts.URL + "/api/v1/auth/logout"))

		var sessionCookie *http.Cookie

		for _, c := range cookies {
			if c.Name == "session_token" {
				sessionCookie = c
				break
			}
		}

		if sessionCookie != nil && sessionCookie.Value != "" {
			t.Fatalf("expected session cookie to be cleared")
		}

		statusCode, refreshResponse, err := refresh(ts.URL, client)
		if err != nil {
			t.Fatalf("couldn't call refresh: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d after logout, got %d", http.StatusUnauthorized, statusCode)
		}

		if refreshResponse.Error == "" {
			t.Fatalf("expected error after logout")
		}
	})

	t.Run("Success - logout without session (no error)", func(t *testing.T) {
		// Create new client without session
		newClient := ts.Client()

		statusCode, err := logout(ts.URL, newClient)
		if err != nil {
			t.Fatalf("couldn't logout: %s", err)
		}

		if statusCode != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, statusCode)
		}
	})

	t.Run("Success - logout with invalid session token (no error)", func(t *testing.T) {
		// Create new client and set invalid session cookie
		newClient := ts.Client()

		req, err := http.NewRequestWithContext(context.Background(), "GET", ts.URL, nil)
		if err != nil {
			t.Fatalf("couldn't create request: %s", err)
		}

		req.AddCookie(&http.Cookie{
			Name:  "session_token",
			Value: "invalid-token-value",
		})
		newClient.Jar.SetCookies(mustParseURL(ts.URL), []*http.Cookie{
			{
				Name:  "session_token",
				Value: "invalid-token-value",
			},
		})

		statusCode, err := logout(ts.URL, newClient)
		if err != nil {
			t.Fatalf("couldn't logout: %s", err)
		}

		if statusCode != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d", http.StatusNoContent, statusCode)
		}
	})

	// Need to login again since we logged out
	_, _, err = login(ts.URL, client, &LoginParams{
		Email:    FirstUserEmail,
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("couldn't login again: %s", err)
	}

	// Test that old token still works (it shouldn't be invalidated by logout)
	t.Run("Old JWT token still works after logout", func(t *testing.T) {
		statusCode, response, err := lookupToken(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't lookup token: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		// Token should still be valid (JWTs are stateless)
		if !response.Result.Valid {
			t.Fatalf("expected JWT token to still be valid after logout")
		}
	})
}

func TestRefreshEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	t.Run("No session token", func(t *testing.T) {
		// Create new client without session
		newClient := ts.Client()

		statusCode, response, err := refresh(ts.URL, newClient)
		if err != nil {
			t.Fatalf("couldn't call refresh: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if response.Error != "No session token" {
			t.Fatalf("expected error 'No session token', got %q", response.Error)
		}
	})

	t.Run("Invalid token encoding", func(t *testing.T) {
		// Create client with invalid base64 token
		newClient := ts.Client()
		newClient.Jar.SetCookies(mustParseURL(ts.URL), []*http.Cookie{
			{
				Name:  "session_token",
				Value: "not-valid-base64!@#$",
			},
		})

		statusCode, response, err := refresh(ts.URL, newClient)
		if err != nil {
			t.Fatalf("couldn't call refresh: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if response.Error != "Invalid token encoding" {
			t.Fatalf("expected error 'Invalid token encoding', got %q", response.Error)
		}
	})

	t.Run("Invalid token length", func(t *testing.T) {
		// Create client with valid base64 but wrong length (16 bytes instead of 32)
		newClient := ts.Client()
		shortToken := make([]byte, 16)
		newClient.Jar.SetCookies(mustParseURL(ts.URL), []*http.Cookie{
			{
				Name:  "session_token",
				Value: base64.URLEncoding.EncodeToString(shortToken),
			},
		})

		statusCode, response, err := refresh(ts.URL, newClient)
		if err != nil {
			t.Fatalf("couldn't call refresh: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if response.Error != "Invalid token length" {
			t.Fatalf("expected error 'Invalid token length', got %q", response.Error)
		}
	})

	t.Run("Invalid session token (not in database)", func(t *testing.T) {
		// Create client with valid base64 (32 bytes) but non-existent session
		newClient := ts.Client()

		fakeToken := make([]byte, 32)
		for i := range fakeToken {
			fakeToken[i] = byte(i)
		}

		newClient.Jar.SetCookies(mustParseURL(ts.URL), []*http.Cookie{
			{
				Name:  "session_token",
				Value: base64.URLEncoding.EncodeToString(fakeToken),
			},
		})

		statusCode, response, err := refresh(ts.URL, newClient)
		if err != nil {
			t.Fatalf("couldn't call refresh: %s", err)
		}

		if statusCode != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, statusCode)
		}

		if response.Error != "Invalid session token" {
			t.Fatalf("expected error 'Invalid session token', got %q", response.Error)
		}
	})
}

func TestSessionLimitPerUser(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, database, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	defer ts.Close()

	client := ts.Client()

	_, err = initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	// Create MaxSessionsPerUser sessions by logging in multiple times
	for i := range server.MaxSessionsPerUser {
		statusCode, _, err := login(ts.URL, client, &LoginParams{
			Email:    FirstUserEmail,
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("couldn't make login request %d: %s", i, err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("login request %d failed with status %d", i, statusCode)
		}
	}

	// Count sessions - should be MaxSessionsPerUser
	sessionCount, err := database.CountSessionsByUser(context.Background(), 1) // User ID 1 is the first user
	if err != nil {
		t.Fatalf("couldn't count sessions: %s", err)
	}

	if sessionCount != server.MaxSessionsPerUser {
		t.Fatalf("expected %d sessions, got %d", server.MaxSessionsPerUser, sessionCount)
	}

	// Login one more time - should still have MaxSessionsPerUser sessions (oldest deleted)
	statusCode, _, err := login(ts.URL, client, &LoginParams{
		Email:    FirstUserEmail,
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("couldn't make final login request: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("final login request failed with status %d", statusCode)
	}

	// Count sessions again - should still be MaxSessionsPerUser
	sessionCount, err = database.CountSessionsByUser(context.Background(), 1)
	if err != nil {
		t.Fatalf("couldn't count sessions after final login: %s", err)
	}

	if sessionCount != server.MaxSessionsPerUser {
		t.Fatalf("expected %d sessions after final login, got %d", server.MaxSessionsPerUser, sessionCount)
	}
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	return u
}
