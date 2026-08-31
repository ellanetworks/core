// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"fmt"
	"net/http"
	"testing"
)

type AuditLog struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	User      string `json:"user"`
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

type UpdateAuditLogPolicyResponseResult struct {
	Message string `json:"message"`
}

func listAuditLogs(url string, client *http.Client, token string, page int, perPage int) (int, *ListAuditLogResponse, error) {
	return apiDo[ListAuditLogResponse](client, "GET", fmt.Sprintf("%s/api/v1/logs/audit?page=%d&per_page=%d", url, page, perPage), token, nil)
}

func TestAPIAuditLogs(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	statusCode, response, err := listAuditLogs(env.Server.URL, client, token, 1, 20)
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

	if response.Result.Items[0].User != FirstUserEmail {
		t.Fatalf("expected first audit log user to be '%s', got %s", FirstUserEmail, response.Result.Items[0].User)
	}

	if response.Result.Items[0].Action != "initialize" {
		t.Fatalf("expected first audit log action to be '%s', got %s", "initialize", response.Result.Items[0].Action)
	}
}

func TestAPIAuditLogsPagination_LargeDataSet(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	// Create 14 users to generate audit logs (15 total with init)
	for i := 1; i <= 14; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		params := &CreateUserParams{
			Email:    email,
			Password: "password123",
			RoleID:   RoleReadOnly,
		}

		statusCode, resp, err := createUser(env.Server.URL, client, token, params)
		if err != nil {
			t.Fatalf("couldn't create user %d: %s", i, err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d when creating user %d, got %d. Error: %s", http.StatusCreated, i, statusCode, resp.Error)
		}
	}

	// Test first page
	statusCode, response, err := listAuditLogs(env.Server.URL, client, token, 1, 5)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 5 {
		t.Fatalf("expected 5 audit logs, got %d", len(response.Result.Items))
	}

	if response.Result.TotalCount != 15 {
		t.Fatalf("expected total_count to be 15, got %d", response.Result.TotalCount)
	}

	if response.Result.Page != 1 {
		t.Fatalf("expected page to be 1, got %d", response.Result.Page)
	}

	if response.Result.PerPage != 5 {
		t.Fatalf("expected per_page to be 5, got %d", response.Result.PerPage)
	}

	if response.Result.Items[0].Details != "User created user: user14@example.com with role: 2" {
		t.Fatalf("expected first audit log details to be correct, got %s", response.Result.Items[0].Details)
	}

	if response.Result.Items[4].Details != "User created user: user10@example.com with role: 2" {
		t.Fatalf("expected last audit log details to be correct, got %s", response.Result.Items[4].Details)
	}

	// Test second page
	statusCode, response, err = listAuditLogs(env.Server.URL, client, token, 2, 5)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 5 {
		t.Fatalf("expected 5 audit logs, got %d", len(response.Result.Items))
	}

	if response.Result.TotalCount != 15 {
		t.Fatalf("expected total_count to be 15, got %d", response.Result.TotalCount)
	}

	if response.Result.Page != 2 {
		t.Fatalf("expected page to be 2, got %d", response.Result.Page)
	}

	if response.Result.PerPage != 5 {
		t.Fatalf("expected per_page to be 5, got %d", response.Result.PerPage)
	}

	if response.Result.Items[0].Details != "User created user: user9@example.com with role: 2" {
		t.Fatalf("expected first audit log details to be correct, got %s", response.Result.Items[0].Details)
	}

	if response.Result.Items[4].Details != "User created user: user5@example.com with role: 2" {
		t.Fatalf("expected last audit log details to be correct, got %s", response.Result.Items[4].Details)
	}

	// Test last page
	statusCode, response, err = listAuditLogs(env.Server.URL, client, token, 3, 5)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result.Items) != 5 {
		t.Fatalf("expected 5 audit logs, got %d", len(response.Result.Items))
	}

	if response.Result.TotalCount != 15 {
		t.Fatalf("expected total_count to be 15, got %d", response.Result.TotalCount)
	}

	if response.Result.Page != 3 {
		t.Fatalf("expected page to be 3, got %d", response.Result.Page)
	}

	if response.Result.PerPage != 5 {
		t.Fatalf("expected per_page to be 5, got %d", response.Result.PerPage)
	}

	if response.Result.Items[0].Details != "User created user: user4@example.com with role: 2" {
		t.Fatalf("expected first audit log details to be correct, got %s", response.Result.Items[0].Details)
	}

	if response.Result.Items[4].Details != "System initialized with first user my.user123@ellanetworks.com" {
		t.Fatalf("expected last audit log details to be correct, got %s", response.Result.Items[4].Details)
	}
}
