package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type GetAuditLogResponseResult struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	IP        string `json:"ip"`
	Details   string `json:"details"`
}

type ListAuditLogResponse struct {
	Result []GetAuditLogResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
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

func listAuditLogs(url string, client *http.Client, token string) (int, *ListAuditLogResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/logs/audit", nil)
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
	ts, _, err := setupServer(dbPath, ReqsPerSec)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	statusCode, response, err := listAuditLogs(ts.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}
	t.Logf("ListAuditLogs response: %v", response)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}
	if len(response.Result) != 2 {
		t.Fatalf("expected 2 audit logs, got %d", len(response.Result))
	}
	if response.Error != "" {
		t.Fatalf("unexpected error :%q", response.Error)
	}

	if response.Result[0].Actor != FirstUserEmail {
		t.Fatalf("expected first audit log actor to be '%s', got %s", FirstUserEmail, response.Result[0].Actor)
	}

	if response.Result[0].Action != "auth_login" {
		t.Fatalf("expected first audit log action to be '%s', got %s", "auth_login", response.Result[0].Action)
	}

	if response.Result[1].Actor != "" {
		t.Fatalf("expected second audit log actor to be '', got %s", response.Result[1].Actor)
	}

	if response.Result[1].Action != "create_user" {
		t.Fatalf("expected second audit log action to be '%s', got %s", "create_user", response.Result[1].Action)
	}
}

func TestAPIAuditLogRetentionPolicyEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")
	ts, _, err := setupServer(dbPath, ReqsPerSec)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer ts.Close()
	client := ts.Client()

	token, err := createFirstUserAndLogin(ts.URL, client)
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

		if response.Result.Days != 30 {
			t.Fatalf("expected default audit log retention policy to be 30 days, got %d", response.Result.Days)
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
	ts, _, err := setupServer(dbPath, ReqsPerSec)
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
