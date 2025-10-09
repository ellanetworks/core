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

type RadioLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	RanID     string `json:"ran_id"`
	Event     string `json:"event"`
	Direction string `json:"direction"`
	Details   string `json:"details"`
}

type ListRadioLogResponseResult struct {
	Items      []RadioLog `json:"items"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalCount int        `json:"total_count"`
}

type ListRadioLogResponse struct {
	Result ListRadioLogResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type GetRadioLogsRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type GetRadioLogRetentionPolicyResponse struct {
	Result *GetRadioLogsRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                     `json:"error,omitempty"`
}

type UpdateRadioLogPolicyResponseResult struct {
	Message string `json:"message"`
}

type UpdateRadioLogRetentionPolicyResponse struct {
	Result *UpdateRadioLogPolicyResponseResult `json:"result,omitempty"`
	Error  string                              `json:"error,omitempty"`
}

type UpdateRadioLogRetentionPolicyParams struct {
	Days int `json:"days"`
}

func listRadioLogs(url string, client *http.Client, token string, page int, perPage int, filters map[string]string) (int, *ListRadioLogResponse, error) {
	var queryParams []string

	queryParams = append(queryParams, fmt.Sprintf("page=%d", page))
	queryParams = append(queryParams, fmt.Sprintf("per_page=%d", perPage))

	for k, v := range filters {
		queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/logs/radio?%s", url, strings.Join(queryParams, "&")), nil)
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

	var radiosLogResponse ListRadioLogResponse

	if err := json.NewDecoder(res.Body).Decode(&radiosLogResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &radiosLogResponse, nil
}

func getRadioLogRetentionPolicy(url string, client *http.Client, token string) (int, *GetRadioLogRetentionPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/logs/radio/retention", nil)
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

	var retentionPolicyResponse GetRadioLogRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&retentionPolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &retentionPolicyResponse, nil
}

func editRadioLogRetentionPolicy(url string, client *http.Client, token string, data *UpdateRadioLogRetentionPolicyParams) (int, *UpdateRadioLogRetentionPolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/logs/radio/retention", strings.NewReader(string(body)))
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
	var updateResponse UpdateRadioLogRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func TestAPIRadioLogs(t *testing.T) {
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
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	statusCode, response, err := listRadioLogs(ts.URL, client, token, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list radio logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 0 {
		t.Fatalf("expected 0 radios log, got %d", len(response.Result.Items))
	}

	if response.Result.Page != 1 {
		t.Fatalf("expected page to be 1, got %d", response.Result.Page)
	}

	if response.Result.PerPage != 10 {
		t.Fatalf("expected per_page to be 10, got %d", response.Result.PerPage)
	}

	if response.Result.TotalCount != 0 {
		t.Fatalf("expected total_count to be 0, got %d", response.Result.TotalCount)
	}

	if response.Error != "" {
		t.Fatalf("unexpected error :%q", response.Error)
	}
}

func TestAPIRadioLogRetentionPolicyEndToEnd(t *testing.T) {
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
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	t.Run("1. Get radios log retention policy", func(t *testing.T) {
		statusCode, response, err := getRadioLogRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get radios log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 7 {
			t.Fatalf("expected default radios log retention policy to be 7 days, got %d", response.Result.Days)
		}
	})

	t.Run("2. Update radios log retention policy", func(t *testing.T) {
		updateRadioLogRetentionPolicyParams := &UpdateRadioLogRetentionPolicyParams{
			Days: 15,
		}
		statusCode, response, err := editRadioLogRetentionPolicy(ts.URL, client, token, updateRadioLogRetentionPolicyParams)
		if err != nil {
			t.Fatalf("couldn't get radios log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Radio log retention policy updated successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("3. Verify updated radios log retention policy", func(t *testing.T) {
		statusCode, response, err := getRadioLogRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get radios log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 15 {
			t.Fatalf("expected updated radios log retention policy to be 15 days, got %d", response.Result.Days)
		}
	})
}

func TestListRadioLogsWithFilter(t *testing.T) {
	tempDir := t.TempDir()

	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, testdb, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := initializeAndRefresh(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	rawEntry1 := `{"timestamp":"2024-10-01T10:00:00Z","level":"info","ran_id":"ran_123","event":"test_event","direction":"inbound","details":"Whatever 1", "raw":"SGVsbG8gd29ybGQh"}`
	rawEntry2 := `{"timestamp":"2024-10-01T11:00:00Z","level":"info","ran_id":"ran_123","event":"another_event","direction":"outbound","details":"Whatever 2", "raw":"SGVsbG8gd29ybGQh"}`
	rawEntry3 := `{"timestamp":"2024-10-01T12:00:00Z","level":"info","ran_id":"ran_456","event":"test_event","direction":"inbound","details":"Whatever 3", "raw":"SGVsbG8gd29ybGQh"}`

	err = testdb.InsertRadioLogJSON(context.Background(), []byte(rawEntry1))
	if err != nil {
		t.Fatalf("couldn't insert radio log: %s", err)
	}

	err = testdb.InsertRadioLogJSON(context.Background(), []byte(rawEntry2))
	if err != nil {
		t.Fatalf("couldn't insert radio log: %s", err)
	}

	err = testdb.InsertRadioLogJSON(context.Background(), []byte(rawEntry3))
	if err != nil {
		t.Fatalf("couldn't insert radio log: %s", err)
	}

	filters := map[string]string{
		"ran_id": "ran_123",
	}

	statusCode, response, err := listRadioLogs(ts.URL, client, token, 1, 10, filters)
	if err != nil {
		t.Fatalf("couldn't list radio logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 2 {
		t.Fatalf("expected 2 radio log, got %d", len(response.Result.Items))
	}
}

func TestUpdateRadioLogRetentionPolicyInvalidInput(t *testing.T) {
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
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	tests := []struct {
		testName string
		days     int
		error    string
	}{
		{
			testName: "Negative days",
			days:     -1,
			error:    "retention days must be greater than 0",
		},
		{
			testName: "0 days",
			days:     0,
			error:    "retention days must be greater than 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			updateParams := &UpdateRadioLogRetentionPolicyParams{
				Days: tt.days,
			}
			statusCode, response, err := editRadioLogRetentionPolicy(ts.URL, client, token, updateParams)
			if err != nil {
				t.Fatalf("couldn't edit radios log retention policy: %s", err)
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
