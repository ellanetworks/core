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

func listRadioLogs(url string, client *http.Client, token string, page int, perPage int) (int, *ListRadioLogResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/logs/radio?page=%d&per_page=%d", url, page, perPage), nil)
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

	statusCode, response, err := listRadioLogs(ts.URL, client, token, 1, 10)
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

		if response.Result.Days != 30 {
			t.Fatalf("expected default radios log retention policy to be 30 days, got %d", response.Result.Days)
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

func TestUpdateRadioLogRetentionPolicyInvalidInput(t *testing.T) {
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
