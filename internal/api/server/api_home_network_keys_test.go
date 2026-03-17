package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type CreateHomeNetworkKeyParams struct {
	KeyIdentifier int    `json:"keyIdentifier"`
	Scheme        string `json:"scheme"`
	PrivateKey    string `json:"privateKey"`
}

type HomeNetworkKeyResponseItem struct {
	ID            int    `json:"id"`
	KeyIdentifier int    `json:"keyIdentifier"`
	Scheme        string `json:"scheme"`
	PublicKey     string `json:"publicKey"`
}

type CreateHomeNetworkKeyResponseResult struct {
	Message string `json:"message"`
}

type CreateHomeNetworkKeyResponse struct {
	Result CreateHomeNetworkKeyResponseResult `json:"result"`
	Error  string                             `json:"error,omitempty"`
}

type DeleteHomeNetworkKeyResponseResult struct {
	Message string `json:"message"`
}

type DeleteHomeNetworkKeyResponse struct {
	Result DeleteHomeNetworkKeyResponseResult `json:"result"`
	Error  string                             `json:"error,omitempty"`
}

func createHomeNetworkKey(url string, client *http.Client, token string, data *CreateHomeNetworkKeyParams) (int, *CreateHomeNetworkKeyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/operator/home-network-keys", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close() //nolint:errcheck

	var resp CreateHomeNetworkKeyResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func deleteHomeNetworkKey(url string, client *http.Client, token string, id int) (int, *DeleteHomeNetworkKeyResponse, error) { //nolint:unparam
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", fmt.Sprintf("%s/api/v1/operator/home-network-keys/%d", url, id), nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close() //nolint:errcheck

	var resp DeleteHomeNetworkKeyResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &resp, nil
}

func TestCreateHomeNetworkKey_ProfileA(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	params := &CreateHomeNetworkKeyParams{
		KeyIdentifier: 1,
		Scheme:        "A",
		PrivateKey:    "5122250214c33e723a5dd523fc145fc05122250214c33e723a5dd523fc145fc0",
	}

	statusCode, resp, err := createHomeNetworkKey(ts.URL, client, token, params)
	if err != nil {
		t.Fatalf("create failed: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d (error: %s)", statusCode, resp.Error)
	}

	// Verify it's listed.
	statusCode, opResp, err := getOperator(ts.URL, client, token)
	if err != nil {
		t.Fatalf("get operator failed: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusCode)
	}

	if len(opResp.Result.HomeNetwork.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(opResp.Result.HomeNetwork.Keys))
	}
}

func TestCreateHomeNetworkKey_ProfileB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	// Valid P-256 private key (32 bytes hex).
	params := &CreateHomeNetworkKeyParams{
		KeyIdentifier: 0,
		Scheme:        "B",
		PrivateKey:    "f1ab1074477ebcce59b97460c83b4071db578ffab54ee4fbc76aeca38e4b7b01",
	}

	statusCode, resp, err := createHomeNetworkKey(ts.URL, client, token, params)
	if err != nil {
		t.Fatalf("create failed: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d (error: %s)", statusCode, resp.Error)
	}

	// Verify the public key is compressed (66 hex chars = 33 bytes).
	statusCode, opResp, err := getOperator(ts.URL, client, token)
	if err != nil {
		t.Fatalf("get operator failed: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusCode)
	}

	for _, k := range opResp.Result.HomeNetwork.Keys {
		if k.Scheme == "B" {
			if len(k.PublicKey) != 66 {
				t.Fatalf("expected 66-char compressed public key, got %d chars", len(k.PublicKey))
			}
		}
	}
}

func TestCreateHomeNetworkKey_InvalidScheme(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	params := &CreateHomeNetworkKeyParams{
		KeyIdentifier: 0,
		Scheme:        "C",
		PrivateKey:    "5122250214c33e723a5dd523fc145fc05122250214c33e723a5dd523fc145fc0",
	}

	statusCode, _, err := createHomeNetworkKey(ts.URL, client, token, params)
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", statusCode)
	}
}

func TestCreateHomeNetworkKey_InvalidPrivateKey(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	params := &CreateHomeNetworkKeyParams{
		KeyIdentifier: 1,
		Scheme:        "A",
		PrivateKey:    "invalidhex",
	}

	statusCode, _, err := createHomeNetworkKey(ts.URL, client, token, params)
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", statusCode)
	}
}

func TestCreateHomeNetworkKey_Duplicate(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	// Default key (A, 0) exists. Try creating another.
	params := &CreateHomeNetworkKeyParams{
		KeyIdentifier: 0,
		Scheme:        "A",
		PrivateKey:    "5122250214c33e723a5dd523fc145fc05122250214c33e723a5dd523fc145fc0",
	}

	statusCode, _, err := createHomeNetworkKey(ts.URL, client, token, params)
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if statusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", statusCode)
	}
}

func TestDeleteHomeNetworkKey(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	// Create a key so we can delete it.
	params := &CreateHomeNetworkKeyParams{
		KeyIdentifier: 1,
		Scheme:        "A",
		PrivateKey:    "5122250214c33e723a5dd523fc145fc05122250214c33e723a5dd523fc145fc0",
	}

	statusCode, _, err := createHomeNetworkKey(ts.URL, client, token, params)
	if err != nil {
		t.Fatalf("create failed: %s", err)
	}

	if statusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", statusCode)
	}

	// List to find the ID.
	statusCode, opResp, err := getOperator(ts.URL, client, token)
	if err != nil {
		t.Fatalf("get operator failed: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusCode)
	}

	// Find the key with identifier 1.
	var keyID int

	for _, k := range opResp.Result.HomeNetwork.Keys {
		if k.KeyIdentifier == 1 {
			keyID = k.ID
			break
		}
	}

	if keyID == 0 {
		t.Fatal("couldn't find key with identifier 1")
	}

	statusCode, _, err = deleteHomeNetworkKey(ts.URL, client, token, keyID)
	if err != nil {
		t.Fatalf("delete failed: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusCode)
	}

	// Verify deleted.
	statusCode, opResp2, err := getOperator(ts.URL, client, token)
	if err != nil {
		t.Fatalf("get operator failed: %s", err)
	}

	_ = statusCode

	if len(opResp2.Result.HomeNetwork.Keys) != 1 {
		t.Fatalf("expected 1 key after deletion, got %d", len(opResp2.Result.HomeNetwork.Keys))
	}
}

func TestDeleteHomeNetworkKey_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	ts, _, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()

	client := newTestClient(ts)

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	statusCode, _, err := deleteHomeNetworkKey(ts.URL, client, token, 9999)
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if statusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", statusCode)
	}
}
