package client_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListAuditLogs_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"id": 1, "timestamp": "2023-10-01T12:00:00Z", "level": "info", "actor": "admin@ellanetworks.com", "action": "login", "ip": "1.2.3.4", "details": "User logged in"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	auditLogs, err := clientObj.ListAuditLogs()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(auditLogs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(auditLogs))
	}

	if auditLogs[0].ID != 1 {
		t.Fatalf("expected audit log ID 1, got %d", auditLogs[0].ID)
	}

	if auditLogs[0].Timestamp != "2023-10-01T12:00:00Z" {
		t.Fatalf("expected timestamp '2023-10-01T12:00:00Z', got '%s'", auditLogs[0].Timestamp)
	}

	if auditLogs[0].Level != "info" {
		t.Fatalf("expected level 'info', got '%s'", auditLogs[0].Level)
	}

	if auditLogs[0].Actor != "admin@ellanetworks.com" {
		t.Fatalf("expected actor 'admin@ellanetworks.com', got '%s'", auditLogs[0].Actor)
	}

	if auditLogs[0].Action != "login" {
		t.Fatalf("expected action 'login', got '%s'", auditLogs[0].Action)
	}

	if auditLogs[0].IP != "1.2.3.4" {
		t.Fatalf("expected IP '1.2.3.4', got '%s'", auditLogs[0].IP)
	}

	if auditLogs[0].Details != "User logged in" {
		t.Fatalf("expected details 'User logged in', got '%s'", auditLogs[0].Details)
	}
}

func TestListAuditLogs_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Internal server error"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	_, err := clientObj.ListAuditLogs()
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetAuditLogRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"days": 30}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	policy, err := clientObj.GetAuditLogRetentionPolicy()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Days != 30 {
		t.Fatalf("expected retention days 30, got %d", policy.Days)
	}
}

func TestGetAuditLogRetentionPolicy_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Internal server error"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	_, err := clientObj.GetAuditLogRetentionPolicy()
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateAuditLogRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Audit log retention policy updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOpts := &client.UpdateAuditLogsRetentionPolicyOptions{
		Days: 60,
	}
	err := clientObj.UpdateAuditLogRetentionPolicy(updateOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateAuditLogRetentionPolicy_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid request body"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOpts := &client.UpdateAuditLogsRetentionPolicyOptions{
		Days: -10,
	}
	err := clientObj.UpdateAuditLogRetentionPolicy(updateOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
