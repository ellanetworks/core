package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type GetSubscriberUsagesRetentionPolicyResponseResult struct {
	Days int `json:"days"`
}

type GetSubscriberUsageRetentionPolicyResponse struct {
	Result *GetSubscriberUsagesRetentionPolicyResponseResult `json:"result,omitempty"`
	Error  string                                            `json:"error,omitempty"`
}

type UpdateSubscriberUsagePolicyResponseResult struct {
	Message string `json:"message"`
}

type UpdateSubscriberUsageRetentionPolicyResponse struct {
	Result *UpdateSubscriberUsagePolicyResponseResult `json:"result,omitempty"`
	Error  string                                     `json:"error,omitempty"`
}

type UpdateSubscriberUsageRetentionPolicyParams struct {
	Days int `json:"days"`
}

func getSubscriberUsageRetentionPolicy(url string, client *http.Client, token string) (int, *GetSubscriberUsageRetentionPolicyResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/subscriber-usage/retention", nil)
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

	var retentionPolicyResponse GetSubscriberUsageRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&retentionPolicyResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &retentionPolicyResponse, nil
}

func editSubscriberUsageRetentionPolicy(url string, client *http.Client, token string, data *UpdateSubscriberUsageRetentionPolicyParams) (int, *UpdateSubscriberUsageRetentionPolicyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/subscriber-usage/retention", strings.NewReader(string(body)))
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
	var updateResponse UpdateSubscriberUsageRetentionPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&updateResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &updateResponse, nil
}

func TestAPISubscriberUsageRetentionPolicyEndToEnd(t *testing.T) {
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

	t.Run("1. Get subscriber usage retention policy", func(t *testing.T) {
		statusCode, response, err := getSubscriberUsageRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 365 {
			t.Fatalf("expected default subscriber usage retention policy to be 365 days, got %d", response.Result.Days)
		}
	})

	t.Run("2. Update subscriber usage retention policy", func(t *testing.T) {
		updateSubscriberUsageRetentionPolicyParams := &UpdateSubscriberUsageRetentionPolicyParams{
			Days: 15,
		}
		statusCode, response, err := editSubscriberUsageRetentionPolicy(ts.URL, client, token, updateSubscriberUsageRetentionPolicyParams)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Message != "Subscriber usage retention policy updated successfully" {
			t.Fatalf("expected success message, got %s", response.Result.Message)
		}
	})

	t.Run("3. Verify updated subscriber usage retention policy", func(t *testing.T) {
		statusCode, response, err := getSubscriberUsageRetentionPolicy(ts.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't get subscriber usage retention policy: %s", err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("unexpected error :%q", response.Error)
		}

		if response.Result.Days != 15 {
			t.Fatalf("expected updated subscriber usage retention policy to be 15 days, got %d", response.Result.Days)
		}
	})
}

func TestUpdateSubscriberUsageRetentionPolicyInvalidInput(t *testing.T) {
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
			updateParams := &UpdateSubscriberUsageRetentionPolicyParams{
				Days: tt.days,
			}
			statusCode, response, err := editSubscriberUsageRetentionPolicy(ts.URL, client, token, updateParams)
			if err != nil {
				t.Fatalf("couldn't edit subscriber usage retention policy: %s", err)
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
