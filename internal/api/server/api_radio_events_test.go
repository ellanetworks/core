package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/dbwriter"
)

type RadioEvent struct {
	ID          int    `json:"id"`
	Timestamp   string `json:"timestamp"`
	Protocol    string `json:"protocol"`
	MessageType string `json:"message_type"`
	Direction   string `json:"direction"`
	Details     string `json:"details"`
}

type ListRadioEventResponseResult struct {
	Items      []RadioEvent `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type ListRadioEventResponse struct {
	Result ListRadioEventResponseResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type GetRadioEventsRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type GetRadioEventRetentionPolicyResponse struct {
	Result *GetRadioEventsRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                       `json:"error,omitempty"`
}

type UpdateRadioEventPolicyResponseResult struct {
	Message string `json:"message"`
}

type UpdateRadioEventRetentionPolicyResponse struct {
	Result *UpdateRadioEventPolicyResponseResult `json:"result,omitempty"`
	Error  string                                `json:"error,omitempty"`
}

type UpdateRadioEventRetentionPolicyParams struct {
	Days int `json:"days"`
}

func listRadioEvents(url string, client *http.Client, token string, page int, perPage int, filters map[string]string) (int, *ListRadioEventResponse, error) {
	var queryParams []string

	queryParams = append(queryParams, fmt.Sprintf("page=%d", page))
	queryParams = append(queryParams, fmt.Sprintf("per_page=%d", perPage))

	for k, v := range filters {
		queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/ran/events?%s", url, strings.Join(queryParams, "&")), nil)
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

	var networksLogResponse ListRadioEventResponse

	if err := json.NewDecoder(res.Body).Decode(&networksLogResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &networksLogResponse, nil
}

func getRadioEventRetentionPolicy(url string, client *http.Client, token string) (int, *GetRadioEventRetentionPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/ran/events/retention", nil)
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

	var retentionPolicyResponse GetRadioEventRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&retentionPolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &retentionPolicyResponse, nil
}

func editRadioEventRetentionPolicy(url string, client *http.Client, token string, data *UpdateRadioEventRetentionPolicyParams) (int, *UpdateRadioEventRetentionPolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/ran/events/retention", strings.NewReader(string(body)))
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

	var updateResponse UpdateRadioEventRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &updateResponse, nil
}

func TestAPIRadioEvents(t *testing.T) {
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

	statusCode, response, err := listRadioEvents(ts.URL, client, token, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list radio events: %s", err)
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

func TestListRadioEventsWithFilter(t *testing.T) {
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

	err = testdb.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
		Timestamp:   "2024-10-01T10:00:00Z",
		Protocol:    "NGAP",
		MessageType: "test_event",
		Direction:   "inbound",
		Details:     "Whatever 1",
		Raw:         []byte("SGVsbG8gd29ybGQh"),
	})
	if err != nil {
		t.Fatalf("couldn't insert radio event: %s", err)
	}

	err = testdb.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
		Timestamp:   "2024-10-01T11:00:00Z",
		Protocol:    "NGAP",
		MessageType: "another_event",
		Direction:   "outbound",
		Details:     "Whatever 2",
		Raw:         []byte("SGVsbG8gd29ybGQh"),
	})
	if err != nil {
		t.Fatalf("couldn't insert radio event: %s", err)
	}

	err = testdb.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
		Timestamp:   "2024-10-01T12:00:00Z",
		Protocol:    "NAS",
		MessageType: "test_event",
		Direction:   "inbound",
		Details:     "Whatever 3",
		Raw:         []byte("SGVsbG8gd29ybGQh"),
	})
	if err != nil {
		t.Fatalf("couldn't insert radio event: %s", err)
	}

	filters := map[string]string{
		"protocol": "NAS",
	}

	statusCode, response, err := listRadioEvents(ts.URL, client, token, 1, 10, filters)
	if err != nil {
		t.Fatalf("couldn't list radio events: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 1 {
		t.Fatalf("expected 1 radio event, got %d", len(response.Result.Items))
	}
}

func TestAPIRadioEventRetentionPolicyEndToEnd(t *testing.T) {
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
		statusCode, response, err := getRadioEventRetentionPolicy(ts.URL, client, token)
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
		updateRadioEventRetentionPolicyParams := &UpdateRadioEventRetentionPolicyParams{
			Days: 15,
		}

		statusCode, response, err := editRadioEventRetentionPolicy(ts.URL, client, token, updateRadioEventRetentionPolicyParams)
		if err != nil {
			t.Fatalf("couldn't get networks log retention policy: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Radio event retention policy updated successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("3. Verify updated networks log retention policy", func(t *testing.T) {
		statusCode, response, err := getRadioEventRetentionPolicy(ts.URL, client, token)
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

func TestUpdateRadioEventRetentionPolicyInvalidInput(t *testing.T) {
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
			updateParams := &UpdateRadioEventRetentionPolicyParams{
				Days: tt.days,
			}

			statusCode, response, err := editRadioEventRetentionPolicy(ts.URL, client, token, updateParams)
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
