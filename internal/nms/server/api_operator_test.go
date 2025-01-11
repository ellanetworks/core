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
	Mcc = "123"
	Mnc = "456"
)

type GetOperatorResponseResult struct {
	Mcc           string   `json:"mcc,omitempty"`
	Mnc           string   `json:"mnc,omitempty"`
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type GetOperatorResponse struct {
	Result GetOperatorResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

type UpdateOperatorParams struct {
	Mcc           string   `json:"mcc,omitempty"`
	Mnc           string   `json:"mnc,omitempty"`
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type UpdateOperatorCodeParams struct {
	OperatorCode string `json:"operatorCode,omitempty"`
}

type UpdateOperatorResponseResult struct {
	Message string `json:"message"`
}

type UpdateOperatorResponse struct {
	Result UpdateOperatorResponseResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type UpdateOperatorCodeResponseResult struct {
	Message string `json:"message"`
}

type UpdateOperatorCodeResponse struct {
	Result UpdateOperatorCodeResponseResult `json:"result"`
	Error  string                           `json:"error,omitempty"`
}

func getOperator(url string, client *http.Client, token string) (int, *GetOperatorResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/operator", nil)
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
	var operatorSliceResponse GetOperatorResponse
	if err := json.NewDecoder(res.Body).Decode(&operatorSliceResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &operatorSliceResponse, nil
}

func updateOperator(url string, client *http.Client, token string, data *UpdateOperatorParams) (int, *UpdateOperatorResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/operator", strings.NewReader(string(body)))
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
	var updateResponse UpdateOperatorResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func updateOperatorCode(url string, client *http.Client, token string, data *UpdateOperatorCodeParams) (int, *UpdateOperatorCodeResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/operator/code", strings.NewReader(string(body)))
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
	var updateResponse UpdateOperatorCodeResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

// This is an end-to-end test for the operators handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestApiOperatorEndToEnd(t *testing.T) {
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

	t.Run("1. Update operator id", func(t *testing.T) {
		updateOperatorParams := &UpdateOperatorParams{
			Mcc: "123",
			Mnc: "456",
			SupportedTacs: []string{
				"001",
				"123",
			},
		}
		statusCode, response, err := updateOperator(ts.URL, client, token, updateOperatorParams)
		if err != nil {
			t.Fatalf("couldn't create operator: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Operator updated successfully" {
			t.Fatalf("expected message %q, got %q", "Operator updated successfully", response.Result.Message)
		}
	})

	t.Run("2. Get operator", func(t *testing.T) {
		statusCode, response, err := getOperator(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get operator: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Mcc != "123" {
			t.Fatalf("expected mcc %s, got %s", "123", response.Result.Mcc)
		}
		if response.Result.Mnc != "456" {
			t.Fatalf("expected mnc %s, got %s", "456", response.Result.Mnc)
		}
		if len(response.Result.SupportedTacs) != 2 {
			t.Fatalf("expected supported TACs of length 2")
		}
		if response.Result.SupportedTacs[0] != "001" {
			t.Fatalf("expected supported TACs first item to be 001, got %s", response.Result.SupportedTacs[0])
		}
		if response.Result.SupportedTacs[1] != "123" {
			t.Fatalf("expected supported TACs first item to be 123, got %s", response.Result.SupportedTacs[1])
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Update operator - no mnc", func(t *testing.T) {
		updateOperatorParams := &UpdateOperatorParams{
			Mcc: "123",
		}
		statusCode, response, err := updateOperator(ts.URL, client, token, updateOperatorParams)
		if err != nil {
			t.Fatalf("couldn't create operator: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "mnc is missing" {
			t.Fatalf("expected error %q, got %q", "mnc is missing", response.Error)
		}
	})

	t.Run("1. Update operator code", func(t *testing.T) {
		updateOperatorCodeParams := &UpdateOperatorCodeParams{
			OperatorCode: "0123456789ABCDEF0123456789ABCDEF",
		}
		statusCode, response, err := updateOperatorCode(ts.URL, client, token, updateOperatorCodeParams)
		if err != nil {
			t.Fatalf("couldn't create operator: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Operator Code updated successfully" {
			t.Fatalf("expected message %q, got %q", "Operator Code updated successfully", response.Result.Message)
		}
	})
}

func TestUpdateOperatorIdInvalidInput(t *testing.T) {
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
		testName      string
		mcc           string
		mnc           string
		supportedTacs []string
		error         string
	}{
		{
			testName:      "Invalid mcc - strings instead of numbers",
			mcc:           "abc",
			mnc:           Mnc,
			supportedTacs: []string{"001"},
			error:         "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName:      "Invalid mcc - too long",
			mcc:           "1234",
			mnc:           Mnc,
			supportedTacs: []string{"001"},
			error:         "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName:      "Invalid mcc - too short",
			mcc:           "12",
			mnc:           Mnc,
			supportedTacs: []string{"001"},
			error:         "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName:      "Invalid mnc - strings instead of numbers",
			mcc:           Mcc,
			mnc:           "abc",
			supportedTacs: []string{"001"},
			error:         "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
		{
			testName:      "Invalid mnc - too long",
			mcc:           Mcc,
			mnc:           "1234",
			supportedTacs: []string{"001"},
			error:         "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
		{
			testName:      "Invalid mnc - too short",
			mcc:           Mcc,
			mnc:           "1",
			supportedTacs: []string{"001"},
			error:         "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
		{
			testName:      "Invalid tac - too short",
			mcc:           Mcc,
			mnc:           Mnc,
			supportedTacs: []string{"001123"},
			error:         "Invalid TAC format. Must be a 3-digit number",
		},
		{
			testName:      "Invalid tac - too short",
			mcc:           Mcc,
			mnc:           Mnc,
			supportedTacs: []string{"00"},
			error:         "Invalid TAC format. Must be a 3-digit number",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			updateOperatorParams := &UpdateOperatorParams{
				Mcc:           tt.mcc,
				Mnc:           tt.mnc,
				SupportedTacs: tt.supportedTacs,
			}
			statusCode, response, err := updateOperator(ts.URL, client, token, updateOperatorParams)
			if err != nil {
				t.Fatalf("couldn't update operator: %s", err)
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

func TestUpdateOperatorCodeInvalidInput(t *testing.T) {
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
		testName     string
		operatorCode string
		error        string
	}{
		{
			testName:     "Invalid operator code - too short",
			operatorCode: "0123456789ABCDEF0123456789ABCDE",
			error:        "Invalid operator code format. Must be a 32-character hexadecimal string.",
		},
		{
			testName:     "Invalid operator code - too long",
			operatorCode: "0123456789ABCDEF0123456789ABCDEF012",
			error:        "Invalid operator code format. Must be a 32-character hexadecimal string.",
		},
		{
			testName:     "Invalid operator code - invalid characters",
			operatorCode: "0123456789ABCDEF0123456789ABCDEF0G",
			error:        "Invalid operator code format. Must be a 32-character hexadecimal string.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			updateOperatorCodeParams := &UpdateOperatorCodeParams{
				OperatorCode: tt.operatorCode,
			}
			statusCode, response, err := updateOperatorCode(ts.URL, client, token, updateOperatorCodeParams)
			if err != nil {
				t.Fatalf("couldn't update operator: %s", err)
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
