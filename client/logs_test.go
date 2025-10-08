package client_test

import (
	"context"
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
			Result:     []byte(`{"items": [{"id": 1, "timestamp": "2023-10-01T12:00:00Z", "level": "info", "actor": "admin@ellanetworks.com", "action": "login", "ip": "1.2.3.4", "details": "User logged in"}], "page": 1, "per_page": 10, "total_count": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	resp, err := clientObj.ListAuditLogs(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(resp.Items))
	}

	if resp.Items[0].ID != 1 {
		t.Fatalf("expected audit log ID 1, got %d", resp.Items[0].ID)
	}

	if resp.Items[0].Timestamp != "2023-10-01T12:00:00Z" {
		t.Fatalf("expected timestamp '2023-10-01T12:00:00Z', got '%s'", resp.Items[0].Timestamp)
	}

	if resp.Items[0].Level != "info" {
		t.Fatalf("expected level 'info', got '%s'", resp.Items[0].Level)
	}

	if resp.Items[0].Actor != "admin@ellanetworks.com" {
		t.Fatalf("expected actor 'admin@ellanetworks.com', got '%s'", resp.Items[0].Actor)
	}

	if resp.Items[0].Action != "login" {
		t.Fatalf("expected action 'login', got '%s'", resp.Items[0].Action)
	}

	if resp.Items[0].IP != "1.2.3.4" {
		t.Fatalf("expected IP '1.2.3.4', got '%s'", resp.Items[0].IP)
	}

	if resp.Items[0].Details != "User logged in" {
		t.Fatalf("expected details 'User logged in', got '%s'", resp.Items[0].Details)
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

	ctx := context.Background()

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	_, err := clientObj.ListAuditLogs(ctx, params)
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

	ctx := context.Background()

	policy, err := clientObj.GetAuditLogRetentionPolicy(ctx)
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

	ctx := context.Background()

	_, err := clientObj.GetAuditLogRetentionPolicy(ctx)
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

	ctx := context.Background()

	err := clientObj.UpdateAuditLogRetentionPolicy(ctx, updateOpts)
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

	ctx := context.Background()

	err := clientObj.UpdateAuditLogRetentionPolicy(ctx, updateOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListSubscriberLogs_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"id": 1, "timestamp": "2023-10-01T12:00:00Z", "level": "info", "imsi": "123456789012345", "event": "PDU Session Establishment Request", "direction": "inbound", "raw": "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA", "details": "{\"pduSessionID\":1}"}], "page": 1, "per_page": 10, "total_count": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	resp, err := clientObj.ListSubscriberLogs(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 subscriber log, got %d", len(resp.Items))
	}

	if resp.Items[0].ID != 1 {
		t.Fatalf("expected subscriber log ID 1, got %d", resp.Items[0].ID)
	}

	if resp.Items[0].Timestamp != "2023-10-01T12:00:00Z" {
		t.Fatalf("expected timestamp '2023-10-01T12:00:00Z', got '%s'", resp.Items[0].Timestamp)
	}

	if resp.Items[0].IMSI != "123456789012345" {
		t.Fatalf("expected IMSI '123456789012345', got '%s'", resp.Items[0].IMSI)
	}

	if resp.Items[0].Level != "info" {
		t.Fatalf("expected level 'info', got '%s'", resp.Items[0].Level)
	}

	if resp.Items[0].Event != "PDU Session Establishment Request" {
		t.Fatalf("expected event 'PDU Session Establishment Request', got '%s'", resp.Items[0].Event)
	}

	if resp.Items[0].Details != "{\"pduSessionID\":1}" {
		t.Fatalf("expected details '{\"pduSessionID\":1}', got '%s'", resp.Items[0].Details)
	}

	if resp.Items[0].Direction != "inbound" {
		t.Fatalf("expected direction 'inbound', got '%s'", resp.Items[0].Direction)
	}

	expectedRaw := "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA"
	if string(resp.Items[0].Raw) != expectedRaw {
		t.Fatalf("expected raw '%s', got '%s'", expectedRaw, string(resp.Items[0].Raw))
	}
}

func TestListSubscriberLogs_Failure(t *testing.T) {
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

	ctx := context.Background()

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	_, err := clientObj.ListSubscriberLogs(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetSubscriberLogsRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"days": 15}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	policy, err := clientObj.GetSubscriberLogRetentionPolicy(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Days != 15 {
		t.Fatalf("expected retention days 15, got %d", policy.Days)
	}
}

func TestGetSubscriberLogsRetentionPolicy_Failure(t *testing.T) {
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

	ctx := context.Background()

	_, err := clientObj.GetSubscriberLogRetentionPolicy(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateSubscriberLogsRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Subscriber log retention policy updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOpts := &client.UpdateSubscriberLogsRetentionPolicyOptions{
		Days: 45,
	}

	ctx := context.Background()

	err := clientObj.UpdateSubscriberLogRetentionPolicy(ctx, updateOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateSubscriberLogsRetentionPolicy_Failure(t *testing.T) {
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

	updateOpts := &client.UpdateSubscriberLogsRetentionPolicyOptions{
		Days: 0,
	}

	ctx := context.Background()

	err := clientObj.UpdateSubscriberLogRetentionPolicy(ctx, updateOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListRadioLogs_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"id": 1, "timestamp": "2023-10-01T12:00:00Z", "level": "info", "ran_id": "ran123", "event": "NGAP Connection Establishment", "direction": "inbound", "raw": "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA", "details": "{\"gnbID\":\"ran123\"}"}], "page": 1, "per_page": 10, "total_count": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	resp, err := clientObj.ListRadioLogs(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 radio log, got %d", len(resp.Items))
	}

	if resp.Items[0].ID != 1 {
		t.Fatalf("expected radio log ID 1, got %d", resp.Items[0].ID)
	}

	if resp.Items[0].Timestamp != "2023-10-01T12:00:00Z" {
		t.Fatalf("expected timestamp '2023-10-01T12:00:00Z', got '%s'", resp.Items[0].Timestamp)
	}

	if resp.Items[0].Level != "info" {
		t.Fatalf("expected level 'info', got '%s'", resp.Items[0].Level)
	}

	if resp.Items[0].RanID != "ran123" {
		t.Fatalf("expected RAN ID 'ran123', got '%s'", resp.Items[0].RanID)
	}

	if resp.Items[0].Event != "NGAP Connection Establishment" {
		t.Fatalf("expected event 'NGAP Connection Establishment', got '%s'", resp.Items[0].Event)
	}

	if resp.Items[0].Details != "{\"gnbID\":\"ran123\"}" {
		t.Fatalf("expected details '{\"gnbID\":\"ran123\"}', got '%s'", resp.Items[0].Details)
	}

	if resp.Items[0].Direction != "inbound" {
		t.Fatalf("expected direction 'inbound', got '%s'", resp.Items[0].Direction)
	}

	expectedRaw := "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA"
	if string(resp.Items[0].Raw) != expectedRaw {
		t.Fatalf("expected raw '%s', got '%s'", expectedRaw, string(resp.Items[0].Raw))
	}
}

func TestListRadioLogs_Failure(t *testing.T) {
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

	ctx := context.Background()

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	_, err := clientObj.ListRadioLogs(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetRadioLogsRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"days": 7}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	policy, err := clientObj.GetRadioLogRetentionPolicy(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Days != 7 {
		t.Fatalf("expected retention days 7, got %d", policy.Days)
	}
}

func TestGetRadioLogsRetentionPolicy_Failure(t *testing.T) {
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

	ctx := context.Background()

	_, err := clientObj.GetRadioLogRetentionPolicy(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
