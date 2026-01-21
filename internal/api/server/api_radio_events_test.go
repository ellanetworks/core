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

func getRadioEvent(url string, client *http.Client, token string, id int) (int, *http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/ran/events/%d", url, id), nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	return res.StatusCode, res, nil
}

func clearRadioEvents(url string, client *http.Client, token string) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/ran/events", nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

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

func TestGetRadioEvent(t *testing.T) {
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

	// Insert a test event
	err = testdb.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
		Timestamp:   "2024-10-01T10:00:00Z",
		Protocol:    "NGAP",
		MessageType: "test_event",
		Direction:   "inbound",
		Details:     "Test event",
		Raw:         []byte("test_raw_data"),
	})
	if err != nil {
		t.Fatalf("couldn't insert radio event: %s", err)
	}

	t.Run("Success - get radio event by ID", func(t *testing.T) {
		statusCode, res, err := getRadioEvent(ts.URL, client, token, 1)
		if err != nil {
			t.Fatalf("couldn't get radio event: %s", err)
		}

		defer func() {
			_ = res.Body.Close()
		}()

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
			t.Fatalf("couldn't decode response: %s", err)
		}

		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected result in response")
		}

		if result["raw"] == nil {
			t.Fatalf("expected raw data in response")
		}
	})

	t.Run("Invalid ID - non-numeric", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/api/v1/ran/events/invalid", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("couldn't do request: %s", err)
		}

		defer func() {
			_ = res.Body.Close()
		}()

		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
		}
	})

	t.Run("Invalid ID - zero", func(t *testing.T) {
		statusCode, res, err := getRadioEvent(ts.URL, client, token, 0)
		if err != nil {
			t.Fatalf("couldn't get radio event: %s", err)
		}

		defer func() {
			_ = res.Body.Close()
		}()

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})

	t.Run("Invalid ID - negative", func(t *testing.T) {
		statusCode, res, err := getRadioEvent(ts.URL, client, token, -1)
		if err != nil {
			t.Fatalf("couldn't get radio event: %s", err)
		}

		defer func() {
			_ = res.Body.Close()
		}()

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})
}

func TestClearRadioEvents(t *testing.T) {
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

	// Insert test events
	for i := 0; i < 5; i++ {
		err = testdb.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
			Timestamp:   fmt.Sprintf("2024-10-01T%02d:00:00Z", 10+i),
			Protocol:    "NGAP",
			MessageType: "test_event",
			Direction:   "inbound",
			Details:     fmt.Sprintf("Test event %d", i+1),
			Raw:         []byte("test_raw_data"),
		})
		if err != nil {
			t.Fatalf("couldn't insert radio event: %s", err)
		}
	}

	// Verify events exist
	statusCode, response, err := listRadioEvents(ts.URL, client, token, 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list radio events: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 5 {
		t.Fatalf("expected 5 radio events before clear, got %d", len(response.Result.Items))
	}

	t.Run("Success - clear all radio events", func(t *testing.T) {
		statusCode, err := clearRadioEvents(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't clear radio events: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		// Verify events are cleared
		statusCode, response, err := listRadioEvents(ts.URL, client, token, 1, 10, nil)
		if err != nil {
			t.Fatalf("couldn't list radio events: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 0 {
			t.Fatalf("expected 0 radio events after clear, got %d", len(response.Result.Items))
		}

		if response.Result.TotalCount != 0 {
			t.Fatalf("expected total_count 0 after clear, got %d", response.Result.TotalCount)
		}
	})
}

func TestListRadioEventsFilters(t *testing.T) {
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

	// Insert various test events
	events := []dbwriter.RadioEvent{
		{
			Timestamp:     "2024-10-01T10:00:00Z",
			Protocol:      "NGAP",
			MessageType:   "InitialUEMessage",
			Direction:     "inbound",
			LocalAddress:  "192.168.1.1:38412",
			RemoteAddress: "192.168.1.100:12345",
			Details:       "Event 1",
			Raw:           []byte("data1"),
		},
		{
			Timestamp:     "2024-10-01T11:00:00Z",
			Protocol:      "NGAP",
			MessageType:   "UplinkNASTransport",
			Direction:     "outbound",
			LocalAddress:  "192.168.1.1:38412",
			RemoteAddress: "192.168.1.101:12346",
			Details:       "Event 2",
			Raw:           []byte("data2"),
		},
		{
			Timestamp:     "2024-10-01T12:00:00Z",
			Protocol:      "NAS",
			MessageType:   "RegistrationRequest",
			Direction:     "inbound",
			LocalAddress:  "192.168.1.1:38412",
			RemoteAddress: "192.168.1.102:12347",
			Details:       "Event 3",
			Raw:           []byte("data3"),
		},
		{
			Timestamp:     "2024-10-01T13:00:00Z",
			Protocol:      "NGAP",
			MessageType:   "InitialUEMessage",
			Direction:     "inbound",
			LocalAddress:  "192.168.1.2:38412",
			RemoteAddress: "192.168.1.103:12348",
			Details:       "Event 4",
			Raw:           []byte("data4"),
		},
	}

	for _, event := range events {
		err = testdb.InsertRadioEvent(context.Background(), &event)
		if err != nil {
			t.Fatalf("couldn't insert radio event: %s", err)
		}
	}

	tests := []struct {
		name           string
		filters        map[string]string
		expectedCount  int
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Filter by direction - inbound",
			filters:        map[string]string{"direction": "inbound"},
			expectedCount:  3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Filter by direction - outbound",
			filters:        map[string]string{"direction": "outbound"},
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Filter by message_type",
			filters:        map[string]string{"message_type": "InitialUEMessage"},
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Filter by local_address",
			filters:        map[string]string{"local_address": "192.168.1.2:38412"},
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Filter by remote_address",
			filters:        map[string]string{"remote_address": "192.168.1.100:12345"},
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Filter by timestamp_from",
			filters:        map[string]string{"timestamp_from": "2024-10-01T11:30:00Z"},
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Filter by timestamp_to",
			filters:        map[string]string{"timestamp_to": "2024-10-01T11:30:00Z"},
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Filter by timestamp range",
			filters: map[string]string{
				"timestamp_from": "2024-10-01T10:30:00Z",
				"timestamp_to":   "2024-10-01T12:30:00Z",
			},
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Multiple filters",
			filters: map[string]string{
				"protocol":  "NGAP",
				"direction": "inbound",
			},
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid direction",
			filters:        map[string]string{"direction": "invalid"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid direction",
		},
		{
			name:           "Invalid timestamp_from format",
			filters:        map[string]string{"timestamp_from": "not-a-timestamp"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid from timestamp",
		},
		{
			name:           "Invalid timestamp_to format",
			filters:        map[string]string{"timestamp_to": "2024-10-01"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid to timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode, response, err := listRadioEvents(ts.URL, client, token, 1, 10, tt.filters)
			if err != nil {
				t.Fatalf("couldn't list radio events: %s", err)
			}

			if statusCode != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, statusCode)
			}

			if tt.expectedStatus == http.StatusOK {
				if len(response.Result.Items) != tt.expectedCount {
					t.Fatalf("expected %d radio events, got %d", tt.expectedCount, len(response.Result.Items))
				}
			} else {
				if response.Error != tt.expectedError {
					t.Fatalf("expected error %q, got %q", tt.expectedError, response.Error)
				}
			}
		})
	}
}

func TestListRadioEventsPagination(t *testing.T) {
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

	// Insert 15 test events
	for i := 0; i < 15; i++ {
		err = testdb.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
			Timestamp:   fmt.Sprintf("2024-10-01T%02d:00:00Z", 10+i),
			Protocol:    "NGAP",
			MessageType: fmt.Sprintf("event_%d", i+1),
			Direction:   "inbound",
			Details:     fmt.Sprintf("Event %d", i+1),
			Raw:         []byte(fmt.Sprintf("data%d", i+1)),
		})
		if err != nil {
			t.Fatalf("couldn't insert radio event: %s", err)
		}
	}

	t.Run("Invalid page - less than 1", func(t *testing.T) {
		statusCode, response, err := listRadioEvents(ts.URL, client, token, 0, 10, nil)
		if err != nil {
			t.Fatalf("couldn't list radio events: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "page must be >= 1" {
			t.Fatalf("expected error 'page must be >= 1', got %q", response.Error)
		}
	})

	t.Run("Invalid per_page - less than 1", func(t *testing.T) {
		statusCode, response, err := listRadioEvents(ts.URL, client, token, 1, 0, nil)
		if err != nil {
			t.Fatalf("couldn't list radio events: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "per_page must be between 1 and 100" {
			t.Fatalf("expected error 'per_page must be between 1 and 100', got %q", response.Error)
		}
	})

	t.Run("Invalid per_page - greater than 100", func(t *testing.T) {
		statusCode, response, err := listRadioEvents(ts.URL, client, token, 1, 101, nil)
		if err != nil {
			t.Fatalf("couldn't list radio events: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "per_page must be between 1 and 100" {
			t.Fatalf("expected error 'per_page must be between 1 and 100', got %q", response.Error)
		}
	})

	t.Run("Page 1 with 10 per page", func(t *testing.T) {
		statusCode, response, err := listRadioEvents(ts.URL, client, token, 1, 10, nil)
		if err != nil {
			t.Fatalf("couldn't list radio events: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 10 {
			t.Fatalf("expected 10 radio events, got %d", len(response.Result.Items))
		}

		if response.Result.TotalCount != 15 {
			t.Fatalf("expected total_count 15, got %d", response.Result.TotalCount)
		}
	})

	t.Run("Page 2 with 10 per page", func(t *testing.T) {
		statusCode, response, err := listRadioEvents(ts.URL, client, token, 2, 10, nil)
		if err != nil {
			t.Fatalf("couldn't list radio events: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result.Items) != 5 {
			t.Fatalf("expected 5 radio events on page 2, got %d", len(response.Result.Items))
		}

		if response.Result.TotalCount != 15 {
			t.Fatalf("expected total_count 15, got %d", response.Result.TotalCount)
		}
	})
}
