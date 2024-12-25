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
	RadioName = "gnb-001"
	Tac       = "001"
)

type GetRadioResponseResult struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type GetRadioResponse struct {
	Result GetRadioResponseResult `json:"result"`
	Error  string                 `json:"error,omitempty"`
}

type CreateRadioParams struct {
	Name string `json:"name"`
	Tac  string `json:"tac"`
}

type CreateRadioResponseResult struct {
	Message string `json:"message"`
}

type CreateRadioResponse struct {
	Result CreateRadioResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

type DeleteRadioResponseResult struct {
	Message string `json:"message"`
}

type DeleteRadioResponse struct {
	Result DeleteRadioResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

func getRadio(url string, client *http.Client, token string, name string) (int, *GetRadioResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/radios/"+name, nil)
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
	var radioResponse GetRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&radioResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &radioResponse, nil
}

func createRadio(url string, client *http.Client, token string, data *CreateRadioParams) (int, *CreateRadioResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/radios", strings.NewReader(string(body)))
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
	var createResponse CreateRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func editRadio(url string, client *http.Client, token string, name string, data *CreateRadioParams) (int, *CreateRadioResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/radios/"+name, strings.NewReader(string(body)))
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
	var createResponse CreateRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteRadio(url string, client *http.Client, token string, name string) (int, *DeleteRadioResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/radios/"+name, nil)
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
	var deleteResponse DeleteRadioResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &deleteResponse, nil
}

// This is an end-to-end test for the radios handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestAPIRadiosEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(db_path)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Create radio", func(t *testing.T) {
		createRadioParams := &CreateRadioParams{
			Name: RadioName,
			Tac:  Tac,
		}
		statusCode, response, err := createRadio(ts.URL, client, token, createRadioParams)
		if err != nil {
			t.Fatalf("couldn't create radio: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Radio created successfully" {
			t.Fatalf("expected message %q, got %q", "Radio created successfully", response.Result.Message)
		}
	})

	t.Run("2. Get radio", func(t *testing.T) {
		statusCode, response, err := getRadio(ts.URL, client, token, RadioName)
		if err != nil {
			t.Fatalf("couldn't get radio: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Name != RadioName {
			t.Fatalf("expected name %s, got %s", RadioName, response.Result.Name)
		}
		if response.Result.Tac != Tac {
			t.Fatalf("expected tac %s, got %s", Tac, response.Result.Tac)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("3. Get radio - id not found", func(t *testing.T) {
		statusCode, response, err := getRadio(ts.URL, client, token, "gnb-002")
		if err != nil {
			t.Fatalf("couldn't get radio: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Radio not found" {
			t.Fatalf("expected error %q, got %q", "Radio not found", response.Error)
		}
	})

	t.Run("4. Create radio - no name", func(t *testing.T) {
		createRadioParams := &CreateRadioParams{
			Tac: Tac,
		}
		statusCode, response, err := createRadio(ts.URL, client, token, createRadioParams)
		if err != nil {
			t.Fatalf("couldn't create radio: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "name is missing" {
			t.Fatalf("expected error %q, got %q", "name is missing", response.Error)
		}
	})

	t.Run("5. Edit radio", func(t *testing.T) {
		createRadioParams := &CreateRadioParams{
			Name: RadioName,
			Tac:  "002",
		}
		statusCode, response, err := editRadio(ts.URL, client, token, RadioName, createRadioParams)
		if err != nil {
			t.Fatalf("couldn't edit radio: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Radio updated successfully" {
			t.Fatalf("expected message %q, got %q", "Radio updated successfully", response.Result.Message)
		}
	})

	t.Run("6. Delete radio - success", func(t *testing.T) {
		statusCode, response, err := deleteRadio(ts.URL, client, token, RadioName)
		if err != nil {
			t.Fatalf("couldn't delete radio: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Radio deleted successfully" {
			t.Fatalf("expected message %q, got %q", "Radio deleted successfully", response.Result.Message)
		}
	})
	t.Run("7. Delete radio - no radio", func(t *testing.T) {
		statusCode, response, err := deleteRadio(ts.URL, client, token, RadioName)
		if err != nil {
			t.Fatalf("couldn't delete radio: %s", err)
		}
		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}
		if response.Error != "Radio not found" {
			t.Fatalf("expected error %q, got %q", "Radio not found", response.Error)
		}
	})
}

func TestCreateRadioInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	db_path := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(db_path)
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
		name  string
		tac   string
		error string
	}{
		{
			name:  "my-gnb-001",
			tac:   "001123",
			error: "Invalid TAC format. Must be a 3-digit number",
		},
		{
			name:  "my-gnb-002",
			tac:   "00",
			error: "Invalid TAC format. Must be a 3-digit number",
		},
		{
			name:  strings.Repeat("a", 257),
			tac:   "001",
			error: "Invalid name format. Must be less than 256 characters",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createRadioParams := &CreateRadioParams{
				Name: tt.name,
				Tac:  tt.tac,
			}
			statusCode, response, err := createRadio(ts.URL, client, token, createRadioParams)
			if err != nil {
				t.Fatalf("couldn't create radio: %s", err)
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
