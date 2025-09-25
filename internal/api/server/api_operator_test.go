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
	Sst = 1
	Sd  = 12345
)

type GetOperatorResponseResult struct {
	ID       GetOperatorIDResponseResult       `json:"id,omitempty"`
	Slice    GetOperatorSliceResponseResult    `json:"slice,omitempty"`
	Tracking GetOperatorTrackingResponseResult `json:"tracking,omitempty"`
}

type GetOperatorResponse struct {
	Result GetOperatorResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

type GetOperatorSliceResponseResult struct {
	Sst int `json:"sst,omitempty"`
	Sd  int `json:"sd,omitempty"`
}

type GetOperatorTrackingResponseResult struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type GetOperatorIDResponseResult struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type GetOperatorSliceResponse struct {
	Result GetOperatorSliceResponseResult `json:"result"`
	Error  string                         `json:"error,omitempty"`
}

type GetOperatorTrackingResponse struct {
	Result GetOperatorTrackingResponseResult `json:"result"`
	Error  string                            `json:"error,omitempty"`
}

type GetOperatorIDResponse struct {
	Result GetOperatorIDResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

type UpdateOperatorSliceParams struct {
	Sst int `json:"sst,omitempty"`
	Sd  int `json:"sd,omitempty"`
}

type UpdateOperatorTrackingParams struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type UpdateOperatorIDParams struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type UpdateOperatorCodeParams struct {
	OperatorCode string `json:"operatorCode,omitempty"`
}

type UpdateOperatorSliceResponseResult struct {
	Message string `json:"message"`
}

type UpdateOperatorTrackingResponseResult struct {
	Message string `json:"message"`
}

type UpdateOperatorIDResponseResult struct {
	Message string `json:"message"`
}

type UpdateOperatorSliceResponse struct {
	Result UpdateOperatorSliceResponseResult `json:"result"`
	Error  string                            `json:"error,omitempty"`
}

type UpdateOperatorTrackingResponse struct {
	Result UpdateOperatorTrackingResponseResult `json:"result"`
	Error  string                               `json:"error,omitempty"`
}

type UpdateOperatorIDResponse struct {
	Result UpdateOperatorIDResponseResult `json:"result"`
	Error  string                         `json:"error,omitempty"`
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
	var operatorResponse GetOperatorResponse
	if err := json.NewDecoder(res.Body).Decode(&operatorResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &operatorResponse, nil
}

func getOperatorSlice(url string, client *http.Client, token string) (int, *GetOperatorSliceResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/operator/slice", nil)
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
	var operatorSliceResponse GetOperatorSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&operatorSliceResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &operatorSliceResponse, nil
}

func getOperatorTracking(url string, client *http.Client, token string) (int, *GetOperatorTrackingResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/operator/tracking", nil)
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
	var operatorTrackingResponse GetOperatorTrackingResponse
	if err := json.NewDecoder(res.Body).Decode(&operatorTrackingResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &operatorTrackingResponse, nil
}

func getOperatorID(url string, client *http.Client, token string) (int, *GetOperatorIDResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/operator/id", nil)
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
	var operatorIDResponse GetOperatorIDResponse
	if err := json.NewDecoder(res.Body).Decode(&operatorIDResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &operatorIDResponse, nil
}

func updateOperatorSlice(url string, client *http.Client, token string, data *UpdateOperatorSliceParams) (int, *UpdateOperatorSliceResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/operator/slice", strings.NewReader(string(body)))
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
	var updateResponse UpdateOperatorSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func updateOperatorTracking(url string, client *http.Client, token string, data *UpdateOperatorTrackingParams) (int, *UpdateOperatorTrackingResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/operator/tracking", strings.NewReader(string(body)))
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
	var updateResponse UpdateOperatorTrackingResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func updateOperatorID(url string, client *http.Client, token string, data *UpdateOperatorIDParams) (int, *UpdateOperatorIDResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/operator/id", strings.NewReader(string(body)))
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
	var updateResponse UpdateOperatorIDResponse
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
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Update operator Slice Information", func(t *testing.T) {
		updateOperatorParams := &UpdateOperatorSliceParams{
			Sst: Sst,
			Sd:  Sd,
		}
		statusCode, response, err := updateOperatorSlice(ts.URL, client, token, updateOperatorParams)
		if err != nil {
			t.Fatalf("couldn't create operator: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Operator slice information updated successfully" {
			t.Fatalf("expected message %q, got %q", "Operator slice information updated successfully", response.Result.Message)
		}
	})

	t.Run("2. Get operator Slice Information", func(t *testing.T) {
		statusCode, response, err := getOperatorSlice(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get operator: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Sst != Sst {
			t.Fatalf("expected sst %d, got %d", Sst, response.Result.Sst)
		}
		if response.Result.Sd != Sd {
			t.Fatalf("expected sd %d, got %d", Sd, response.Result.Sd)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("4. Update operator slice - no sst", func(t *testing.T) {
		updateOperatorParams := &UpdateOperatorSliceParams{
			Sd: 123,
		}
		statusCode, response, err := updateOperatorSlice(ts.URL, client, token, updateOperatorParams)
		if err != nil {
			t.Fatalf("couldn't create operator: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "sst is missing" {
			t.Fatalf("expected error %q, got %q", "sst is missing", response.Error)
		}
	})

	t.Run("5. Update operator tracking", func(t *testing.T) {
		updateOperatorTrackingParams := &UpdateOperatorTrackingParams{
			SupportedTacs: []string{
				"001",
				"123",
			},
		}

		statusCode, response, err := updateOperatorTracking(ts.URL, client, token, updateOperatorTrackingParams)
		if err != nil {
			t.Fatalf("couldn't create operator: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Operator tracking information updated successfully" {
			t.Fatalf("expected message %q, got %q", "Operator tracking information updated successfully", response.Result.Message)
		}
	})

	t.Run("6. Get operator tracking", func(t *testing.T) {
		statusCode, response, err := getOperatorTracking(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get operator: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
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

	t.Run("7. Update operator tracking - no supportedTacs", func(t *testing.T) {
		updateOperatorTrackingParams := &UpdateOperatorTrackingParams{}
		statusCode, response, err := updateOperatorTracking(ts.URL, client, token, updateOperatorTrackingParams)
		if err != nil {
			t.Fatalf("couldn't create operator: %s", err)
		}
		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
		if response.Error != "supportedTacs is missing" {
			t.Fatalf("expected error %q, got %q", "supportedTacs is missing", response.Error)
		}
	})

	t.Run("8. Update operator Id", func(t *testing.T) {
		updateOperatorIDParams := &UpdateOperatorIDParams{
			Mcc: Mcc,
			Mnc: Mnc,
		}
		statusCode, response, err := updateOperatorID(ts.URL, client, token, updateOperatorIDParams)
		if err != nil {
			t.Fatalf("couldn't update operator: %s", err)
		}
		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
		if response.Result.Message != "Operator ID updated successfully" {
			t.Fatalf("expected message %q, got %q", "Operator ID updated successfully", response.Result.Message)
		}
	})

	t.Run("9. Get operator Id", func(t *testing.T) {
		statusCode, response, err := getOperatorID(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get operator Id: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.Mcc != Mcc {
			t.Fatalf("expected mcc %q, got %q", Mcc, response.Result.Mcc)
		}
		if response.Result.Mnc != Mnc {
			t.Fatalf("expected mnc %q, got %q", Mnc, response.Result.Mnc)
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})

	t.Run("10. Update operator code", func(t *testing.T) {
		updateOperatorCodeParams := &UpdateOperatorCodeParams{
			OperatorCode: "0123456789ABCDEF0123456789ABCDEF",
		}
		statusCode, response, err := updateOperatorCode(ts.URL, client, token, updateOperatorCodeParams)
		if err != nil {
			t.Fatalf("couldn't update operator code: %s", err)
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

	t.Run("11. Get Operator", func(t *testing.T) {
		statusCode, response, err := getOperator(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get operator: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
		if response.Result.ID.Mcc != Mcc {
			t.Fatalf("expected mcc %q, got %q", Mcc, response.Result.ID.Mcc)
		}
		if response.Result.ID.Mnc != Mnc {
			t.Fatalf("expected mnc %q, got %q", Mnc, response.Result.ID.Mnc)
		}
		if response.Result.Slice.Sst != 1 {
			t.Fatalf("expected sst %d, got %d", 1, response.Result.Slice.Sst)
		}
		if response.Result.Slice.Sd != 12345 {
			t.Fatalf("expected sd %d, got %d", 12345, response.Result.Slice.Sd)
		}
		if len(response.Result.Tracking.SupportedTacs) != 2 {
			t.Fatalf("expected supported TACs of length 2")
		}
		if response.Result.Tracking.SupportedTacs[0] != "001" {
			t.Fatalf("expected supported TACs first item to be 001, got %s", response.Result.Tracking.SupportedTacs[0])
		}
		if response.Result.Tracking.SupportedTacs[1] != "123" {
			t.Fatalf("expected supported TACs first item to be 123, got %s", response.Result.Tracking.SupportedTacs[1])
		}
		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}
	})
}

func TestUpdateOperatorSliceInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		testName string
		sst      int
		sd       int
		error    string
	}{
		{
			testName: "Invalid sst - negative",
			sst:      -1,
			sd:       Sd,
			error:    "Invalid SST format. Must be an 8-bit integer",
		},
		{
			testName: "Invalid sst - too big",
			sst:      256,
			sd:       Sd,
			error:    "Invalid SST format. Must be an 8-bit integer",
		},
		{
			testName: "Invalid sd - negative",
			sst:      Sst,
			sd:       -1,
			error:    "Invalid SD format. Must be a 24-bit integer",
		},
		{
			testName: "Invalid sd - too big",
			sst:      Sst,
			sd:       16777216,
			error:    "Invalid SD format. Must be a 24-bit integer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			updateOperatorParams := &UpdateOperatorSliceParams{
				Sst: tt.sst,
				Sd:  tt.sd,
			}
			statusCode, response, err := updateOperatorSlice(ts.URL, client, token, updateOperatorParams)
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

func TestUpdateOperatorTrackingInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		testName      string
		supportedTacs []string
		error         string
	}{
		{
			testName:      "Invalid tac - too short",
			supportedTacs: []string{"001123"},
			error:         "Invalid TAC format. Must be a 3-digit number",
		},
		{
			testName:      "Invalid tac - too short",
			supportedTacs: []string{"00"},
			error:         "Invalid TAC format. Must be a 3-digit number",
		},
		{
			testName: "Too many TACs",
			supportedTacs: []string{
				"001", "002", "003", "004", "005", "006", "007", "008",
				"009", "010", "011", "012", "013", "014", "015", "016",
			},
			error: "Too many supported TACs. Maximum is 12",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			updateOperatorParams := &UpdateOperatorTrackingParams{
				SupportedTacs: tt.supportedTacs,
			}
			statusCode, response, err := updateOperatorTracking(ts.URL, client, token, updateOperatorParams)
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

func TestUpdateOperatorIDInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		testName string
		mcc      string
		mnc      string
		error    string
	}{
		{
			testName: "Invalid mcc - strings instead of numbers",
			mcc:      "abc",
			mnc:      Mnc,

			error: "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName: "Invalid mcc - too long",
			mcc:      "1234",
			mnc:      Mnc,
			error:    "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName: "Invalid mcc - too short",
			mcc:      "12",
			mnc:      Mnc,
			error:    "Invalid mcc format. Must be a 3-decimal digit.",
		},
		{
			testName: "Invalid mnc - strings instead of numbers",
			mcc:      Mcc,
			mnc:      "abc",
			error:    "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
		{
			testName: "Invalid mnc - too long",
			mcc:      Mcc,
			mnc:      "1234",
			error:    "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
		{
			testName: "Invalid mnc - too short",
			mcc:      Mcc,
			mnc:      "1",
			error:    "Invalid mnc format. Must be a 2 or 3-decimal digit.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			updateOperatorParams := &UpdateOperatorIDParams{
				Mcc: tt.mcc,
				Mnc: tt.mnc,
			}
			statusCode, response, err := updateOperatorID(ts.URL, client, token, updateOperatorParams)
			if err != nil {
				t.Fatalf("couldn't update operator ID: %s", err)
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
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
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
