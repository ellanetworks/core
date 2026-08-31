// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"testing"
)

type RetentionPolicyParams struct {
	Days int `json:"days"`
}

type RetentionPolicyResponseResult struct {
	Days    int    `json:"days"`
	Message string `json:"message"`
}

type RetentionPolicyResponse struct {
	Result RetentionPolicyResponseResult `json:"result"`
	Error  string                        `json:"error,omitempty"`
}

var retentionPolicies = []struct {
	name        string
	path        string
	defaultDays int
	updatedMsg  string
}{
	{
		name:        "audit logs",
		path:        "/api/v1/logs/audit/retention",
		defaultDays: 7,
		updatedMsg:  "Audit log retention policy updated successfully",
	},
	{
		name:        "radio events",
		path:        "/api/v1/ran/events/retention",
		defaultDays: 7,
		updatedMsg:  "Radio event retention policy updated successfully",
	},
	{
		name:        "subscriber usage",
		path:        "/api/v1/subscriber-usage/retention",
		defaultDays: 365,
		updatedMsg:  "Subscriber usage retention policy updated successfully",
	},
}

func getRetentionPolicy(url string, client *http.Client, token, path string) (int, *RetentionPolicyResponse, error) {
	return apiDo[RetentionPolicyResponse](client, "GET", url+path, token, nil)
}

func editRetentionPolicy(url string, client *http.Client, token, path string, days int) (int, *RetentionPolicyResponse, error) {
	return apiDo[RetentionPolicyResponse](client, "PUT", url+path, token, &RetentionPolicyParams{Days: days})
}

func assertRetentionDays(t *testing.T, url string, client *http.Client, token, path string, want int) {
	t.Helper()

	statusCode, response, err := getRetentionPolicy(url, client, token, path)
	if err != nil {
		t.Fatalf("couldn't get retention policy: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Error != "" {
		t.Fatalf("unexpected error: %q", response.Error)
	}

	if response.Result.Days != want {
		t.Fatalf("retention policy = %d days, want %d", response.Result.Days, want)
	}
}

func TestAPIRetentionPoliciesEndToEnd(t *testing.T) {
	for _, rp := range retentionPolicies {
		t.Run(rp.name, func(t *testing.T) {
			env, client, token := newAuthedTestEnv(t)

			assertRetentionDays(t, env.Server.URL, client, token, rp.path, rp.defaultDays)

			statusCode, response, err := editRetentionPolicy(env.Server.URL, client, token, rp.path, 15)
			if err != nil {
				t.Fatalf("couldn't update retention policy: %s", err)
			}

			if statusCode != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
			}

			if response.Error != "" {
				t.Fatalf("unexpected error: %q", response.Error)
			}

			if response.Result.Message != rp.updatedMsg {
				t.Fatalf("message = %q, want %q", response.Result.Message, rp.updatedMsg)
			}

			assertRetentionDays(t, env.Server.URL, client, token, rp.path, 15)
		})
	}
}

func TestUpdateRetentionPolicyInvalidInput(t *testing.T) {
	const wantError = "retention days must be greater than 0"

	for _, rp := range retentionPolicies {
		t.Run(rp.name, func(t *testing.T) {
			env, client, token := newAuthedTestEnv(t)

			for _, days := range []int{-1, 0} {
				statusCode, response, err := editRetentionPolicy(env.Server.URL, client, token, rp.path, days)
				if err != nil {
					t.Fatalf("couldn't edit retention policy: %s", err)
				}

				if statusCode != http.StatusBadRequest {
					t.Errorf("days=%d: expected status %d, got %d", days, http.StatusBadRequest, statusCode)
				}

				if response.Error != wantError {
					t.Errorf("days=%d: expected error %q, got %q", days, wantError, response.Error)
				}
			}
		})
	}
}
