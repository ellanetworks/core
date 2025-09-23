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

type SubscriberLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	IMSI      string `json:"imsi"`
	Event     string `json:"event"`
	Details   string `json:"details"`
}

type ListSubscriberLogResponseResult struct {
	Items      []SubscriberLog `json:"items"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	TotalCount int             `json:"total_count"`
}

type ListSubscriberLogResponse struct {
	Result ListSubscriberLogResponseResult `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type GetSubscriberLogsRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type GetSubscriberLogRetentionPolicyResponse struct {
	Result *GetSubscriberLogsRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                          `json:"error,omitempty"`
}

type UpdateSubscriberLogPolicyResponseResult struct {
	Message string `json:"message"`
}

type UpdateSubscriberLogRetentionPolicyResponse struct {
	Result *UpdateSubscriberLogPolicyResponseResult `json:"result,omitempty"`
	Error  string                                   `json:"error,omitempty"`
}

type UpdateSubscriberLogRetentionPolicyParams struct {
	Days int `json:"days"`
}

func listSubscriberLogs(url string, client *http.Client, token string, page int, perPage int) (int, *ListSubscriberLogResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/logs/subscriber?page=%d&per_page=%d", url, page, perPage), nil)
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

	var subscribersLogResponse ListSubscriberLogResponse

	if err := json.NewDecoder(res.Body).Decode(&subscribersLogResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &subscribersLogResponse, nil
}

func getSubscriberLogRetentionPolicy(url string, client *http.Client, token string) (int, *GetSubscriberLogRetentionPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/logs/subscriber/retention", nil)
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

	var retentionPolicyResponse GetSubscriberLogRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&retentionPolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &retentionPolicyResponse, nil
}

func editSubscriberLogRetentionPolicy(url string, client *http.Client, token string, data *UpdateSubscriberLogRetentionPolicyParams) (int, *UpdateSubscriberLogRetentionPolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/logs/subscriber/retention", strings.NewReader(string(body)))
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
	var updateResponse UpdateSubscriberLogRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func TestAPISubscriberLogs(t *testing.T) {
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

	statusCode, response, err := listSubscriberLogs(ts.URL, client, token, 1, 10)
	if err != nil {
		t.Fatalf("couldn't list subscriber logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 0 {
		t.Fatalf("expected 0 subscribers log, got %d", len(response.Result.Items))
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

func TestAPISubscriberLogRetentionPolicyEndToEnd(t *testing.T) {
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

	t.Run("1. Get subscribers log retention policy", func(t *testing.T) {
		statusCode, response, err := getSubscriberLogRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get subscribers log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 30 {
			t.Fatalf("expected default subscribers log retention policy to be 30 days, got %d", response.Result.Days)
		}
	})

	t.Run("2. Update subscribers log retention policy", func(t *testing.T) {
		updateSubscriberLogRetentionPolicyParams := &UpdateSubscriberLogRetentionPolicyParams{
			Days: 15,
		}
		statusCode, response, err := editSubscriberLogRetentionPolicy(ts.URL, client, token, updateSubscriberLogRetentionPolicyParams)
		if err != nil {
			t.Fatalf("couldn't get subscribers log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Subscriber log retention policy updated successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("3. Verify updated subscribers log retention policy", func(t *testing.T) {
		statusCode, response, err := getSubscriberLogRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get subscribers log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 15 {
			t.Fatalf("expected updated subscribers log retention policy to be 15 days, got %d", response.Result.Days)
		}
	})
}

func TestUpdateSubscriberLogRetentionPolicyInvalidInput(t *testing.T) {
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
			updateParams := &UpdateSubscriberLogRetentionPolicyParams{
				Days: tt.days,
			}
			statusCode, response, err := editSubscriberLogRetentionPolicy(ts.URL, client, token, updateParams)
			if err != nil {
				t.Fatalf("couldn't edit subscribers log retention policy: %s", err)
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
