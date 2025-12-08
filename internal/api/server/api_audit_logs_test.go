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

type AuditLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	IP        string `json:"ip"`
	Details   string `json:"details"`
}

type ListAuditLogResponseResult struct {
	Items      []AuditLog `json:"items"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalCount int        `json:"total_count"`
}

type ListAuditLogResponse struct {
	Result ListAuditLogResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type GetAuditLogsRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type GetAuditLogRetentionPolicyResponse struct {
	Result *GetAuditLogsRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                     `json:"error,omitempty"`
}

type UpdateAuditLogPolicyResponseResult struct {
	Message string `json:"message"`
}

type UpdateAuditLogRetentionPolicyResponse struct {
	Result *UpdateAuditLogPolicyResponseResult `json:"result,omitempty"`
	Error  string                              `json:"error,omitempty"`
}

type UpdateAuditLogRetentionPolicyParams struct {
	Days int `json:"days"`
}

func listAuditLogs(url string, client *http.Client, token string, page int, perPage int) (int, *ListAuditLogResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s/api/v1/logs/audit?page=%d&per_page=%d", url, page, perPage), nil)
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

	var auditLogResponse ListAuditLogResponse
	if err := json.NewDecoder(res.Body).Decode(&auditLogResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &auditLogResponse, nil
}

func getAuditLogRetentionPolicy(url string, client *http.Client, token string) (int, *GetAuditLogRetentionPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/logs/audit/retention", nil)
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

	var retentionPolicyResponse GetAuditLogRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&retentionPolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &retentionPolicyResponse, nil
}

func editAuditLogRetentionPolicy(url string, client *http.Client, token string, data *UpdateAuditLogRetentionPolicyParams) (int, *UpdateAuditLogRetentionPolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/logs/audit/retention", strings.NewReader(string(body)))
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
	var updateResponse UpdateAuditLogRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func TestAPIAuditLogs(t *testing.T) {
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

	statusCode, response, err := listAuditLogs(ts.URL, client, token, 1, 20)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(response.Result.Items))
	}

	if response.Error != "" {
		t.Fatalf("unexpected error :%q", response.Error)
	}

	if response.Result.Items[0].Actor != FirstUserEmail {
		t.Fatalf("expected first audit log actor to be '%s', got %s", FirstUserEmail, response.Result.Items[0].Actor)
	}

	if response.Result.Items[0].Action != "initialize" {
		t.Fatalf("expected first audit log action to be '%s', got %s", "initialize", response.Result.Items[0].Action)
	}
}

func TestAPIAuditLogsPagination_LargeDataSet(t *testing.T) {
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

	// Create 49 users to generate audit logs
	for i := 1; i <= 49; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		params := &CreateUserParams{
			Email:    email,
			Password: "password123",
			RoleID:   RoleReadOnly,
		}

		statusCode, resp, err := createUser(ts.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create user %d: %s", i, err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d when creating user %d, got %d. Error: %s", http.StatusCreated, i, statusCode, resp.Error)
		}
	}

	// Test first page
	statusCode, response, err := listAuditLogs(ts.URL, client, token, 1, 10)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 10 {
		t.Fatalf("expected 10 audit logs, got %d", len(response.Result.Items))
	}

	if response.Result.TotalCount != 50 {
		t.Fatalf("expected total_count to be 50, got %d", response.Result.TotalCount)
	}

	if response.Result.Page != 1 {
		t.Fatalf("expected page to be 1, got %d", response.Result.Page)
	}

	if response.Result.PerPage != 10 {
		t.Fatalf("expected per_page to be 10, got %d", response.Result.PerPage)
	}

	if response.Result.Items[0].Details != "User created user: user49@example.com with role: 2" {
		t.Fatalf("expected first audit log details to be correct, got %s", response.Result.Items[0].Details)
	}

	if response.Result.Items[9].Details != "User created user: user40@example.com with role: 2" {
		t.Fatalf("expected last audit log details to be correct, got %s", response.Result.Items[9].Details)
	}

	// Test second page
	statusCode, response, err = listAuditLogs(ts.URL, client, token, 2, 10)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 10 {
		t.Fatalf("expected 10 audit logs, got %d", len(response.Result.Items))
	}

	if response.Result.TotalCount != 50 {
		t.Fatalf("expected total_count to be 50, got %d", response.Result.TotalCount)
	}

	if response.Result.Page != 2 {
		t.Fatalf("expected page to be 2, got %d", response.Result.Page)
	}

	if response.Result.PerPage != 10 {
		t.Fatalf("expected per_page to be 10, got %d", response.Result.PerPage)
	}

	if response.Result.Items[0].Details != "User created user: user39@example.com with role: 2" {
		t.Fatalf("expected first audit log details to be correct, got %s", response.Result.Items[0].Details)
	}

	if response.Result.Items[9].Details != "User created user: user30@example.com with role: 2" {
		t.Fatalf("expected last audit log details to be correct, got %s", response.Result.Items[9].Details)
	}

	// Test last page
	statusCode, response, err = listAuditLogs(ts.URL, client, token, 5, 10)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 10 {
		t.Fatalf("expected 10 audit logs, got %d", len(response.Result.Items))
	}

	if response.Result.TotalCount != 50 {
		t.Fatalf("expected total_count to be 50, got %d", response.Result.TotalCount)
	}

	if response.Result.Page != 5 {
		t.Fatalf("expected page to be 5, got %d", response.Result.Page)
	}

	if response.Result.PerPage != 10 {
		t.Fatalf("expected per_page to be 10, got %d", response.Result.PerPage)
	}

	if response.Result.Items[0].Details != "User created user: user9@example.com with role: 2" {
		t.Fatalf("expected first audit log details to be correct, got %s", response.Result.Items[0].Details)
	}

	if response.Result.Items[9].Details != "System initialized with first user my.user123@ellanetworks.com" {
		t.Fatalf("expected last audit log details to be correct, got %s", response.Result.Items[9].Details)
	}
}

func TestAPIAuditLogRetentionPolicyEndToEnd(t *testing.T) {
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

	t.Run("1. Get audit log retention policy", func(t *testing.T) {
		statusCode, response, err := getAuditLogRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get audit log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 7 {
			t.Fatalf("expected default audit log retention policy to be 7 days, got %d", response.Result.Days)
		}
	})

	t.Run("2. Update audit log retention policy", func(t *testing.T) {
		updateAuditLogRetentionPolicyParams := &UpdateAuditLogRetentionPolicyParams{
			Days: 15,
		}
		statusCode, response, err := editAuditLogRetentionPolicy(ts.URL, client, token, updateAuditLogRetentionPolicyParams)
		if err != nil {
			t.Fatalf("couldn't get audit log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Audit log retention policy updated successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("3. Verify updated audit log retention policy", func(t *testing.T) {
		statusCode, response, err := getAuditLogRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get audit log retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 15 {
			t.Fatalf("expected updated audit log retention policy to be 15 days, got %d", response.Result.Days)
		}
	})
}

func TestUpdateAuditLogRetentionPolicyInvalidInput(t *testing.T) {
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
			updateParams := &UpdateAuditLogRetentionPolicyParams{
				Days: tt.days,
			}
			statusCode, response, err := editAuditLogRetentionPolicy(ts.URL, client, token, updateParams)
			if err != nil {
				t.Fatalf("couldn't edit audit log retention policy: %s", err)
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
