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

type NetworkLog struct {
	ID          int    `json:"id"`
	Timestamp   string `json:"timestamp"`
	Protocol    string `json:"protocol"`
	MessageType string `json:"message_type"`
	Direction   string `json:"direction"`
	Details     string `json:"details"`
}

type ListNetworkLogResponseResult struct {
	Items      []NetworkLog `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type ListNetworkLogResponse struct {
	Result ListNetworkLogResponseResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type GetNetworkLogsRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type GetNetworkLogRetentionPolicyResponse struct {
	Result *GetNetworkLogsRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                       `json:"error,omitempty"`
}

type UpdateNetworkLogPolicyResponseResult struct {
	Message string `json:"message"`
}

type UpdateNetworkLogRetentionPolicyResponse struct {
	Result *UpdateNetworkLogPolicyResponseResult `json:"result,omitempty"`
	Error  string                                `json:"error,omitempty"`
}

type UpdateNetworkLogRetentionPolicyParams struct {
	Days int `json:"days"`
}

func listNetworkLogs(url string, client *http.Client, token string, page int, perPage int, filters map[string]string) (int, *ListNetworkLogResponse, error) {
	var queryParams []string

	queryParams = append(queryParams, fmt.Sprintf("page=%d", page))
	queryParams = append(queryParams, fmt.Sprintf("per_page=%d", perPage))

	for k, v := range filters {
		queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/logs/network?%s", url, strings.Join(queryParams, "&")), nil)
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

	var networksLogResponse ListNetworkLogResponse

	if err := json.NewDecoder(res.Body).Decode(&networksLogResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &networksLogResponse, nil
}

func getNetworkLogRetentionPolicy(url string, client *http.Client, token string) (int, *GetNetworkLogRetentionPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/logs/network/retention", nil)
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

	var retentionPolicyResponse GetNetworkLogRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&retentionPolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &retentionPolicyResponse, nil
}

func editNetworkLogRetentionPolicy(url string, client *http.Client, token string, data *UpdateNetworkLogRetentionPolicyParams) (int, *UpdateNetworkLogRetentionPolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/logs/network/retention", strings.NewReader(string(body)))
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
	var updateResponse UpdateNetworkLogRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func TestAPINetworkLogs(t *testing.T) {
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

	statusCode, response, err := listNetworkLogs(ts.URL, client, token, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list network logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 0 {
		t.Fatalf("expected 0 networks log, got %d", len(response.Result.Items))
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

func TestListNetworkLogsWithFilter(t *testing.T) {
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

	rawEntry1 := `{"timestamp":"2024-10-01T10:00:00Z","protocol":"NGAP","message_type":"test_event","direction":"inbound","details":"Whatever 1", "raw":"SGVsbG8gd29ybGQh"}`
	rawEntry2 := `{"timestamp":"2024-10-01T11:00:00Z","protocol":"NGAP","message_type":"another_event","direction":"outbound","details":"Whatever 2", "raw":"SGVsbG8gd29ybGQh"}`
	rawEntry3 := `{"timestamp":"2024-10-01T12:00:00Z","protocol":"NAS","message_type":"test_event","direction":"inbound","details":"Whatever 3", "raw":"SGVsbG8gd29ybGQh"}`

	err = testdb.InsertNetworkLogJSON(context.Background(), []byte(rawEntry1))
	if err != nil {
		t.Fatalf("couldn't insert network log: %s", err)
	}

	err = testdb.InsertNetworkLogJSON(context.Background(), []byte(rawEntry2))
	if err != nil {
		t.Fatalf("couldn't insert network log: %s", err)
	}

	err = testdb.InsertNetworkLogJSON(context.Background(), []byte(rawEntry3))
	if err != nil {
		t.Fatalf("couldn't insert network log: %s", err)
	}

	filters := map[string]string{
		"protocol": "NAS",
	}

	statusCode, response, err := listNetworkLogs(ts.URL, client, token, 1, 10, filters)
	if err != nil {
		t.Fatalf("couldn't list network logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 1 {
		t.Fatalf("expected 1 network log, got %d", len(response.Result.Items))
	}
}

func TestAPINetworkLogRetentionPolicyEndToEnd(t *testing.T) {
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

	t.Run("1. Get networks log retention policy", func(t *testing.T) {
		statusCode, response, err := getNetworkLogRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get networks log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 7 {
			t.Fatalf("expected default networks log retention policy to be 7 days, got %d", response.Result.Days)
		}
	})

	t.Run("2. Update networks log retention policy", func(t *testing.T) {
		updateNetworkLogRetentionPolicyParams := &UpdateNetworkLogRetentionPolicyParams{
			Days: 15,
		}
		statusCode, response, err := editNetworkLogRetentionPolicy(ts.URL, client, token, updateNetworkLogRetentionPolicyParams)
		if err != nil {
			t.Fatalf("couldn't get networks log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Network log retention policy updated successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("3. Verify updated networks log retention policy", func(t *testing.T) {
		statusCode, response, err := getNetworkLogRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get networks log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 15 {
			t.Fatalf("expected updated networks log retention policy to be 15 days, got %d", response.Result.Days)
		}
	})
}

func TestUpdateNetworkLogRetentionPolicyInvalidInput(t *testing.T) {
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
			updateParams := &UpdateNetworkLogRetentionPolicyParams{
				Days: tt.days,
			}
			statusCode, response, err := editNetworkLogRetentionPolicy(ts.URL, client, token, updateParams)
			if err != nil {
				t.Fatalf("couldn't edit networks log retention policy: %s", err)
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
