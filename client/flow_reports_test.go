// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListFlowReports_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"id": 1, "subscriber_id": "001010100000001", "source_ip": "10.0.0.1", "destination_ip": "8.8.8.8", "source_port": 12345, "destination_port": 53, "protocol": 17, "packets": 100, "bytes": 50000, "start_time": "2026-02-20T10:00:00Z", "end_time": "2026-02-20T10:05:00Z"}], "page": 1, "per_page": 25, "total_count": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListFlowReportsParams{
		Page:    1,
		PerPage: 25,
	}

	resp, err := clientObj.ListFlowReports(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 flow report, got %d", len(resp.Items))
	}

	if resp.Items[0].ID != 1 {
		t.Fatalf("expected flow report ID 1, got %d", resp.Items[0].ID)
	}

	if resp.Items[0].SubscriberID != "001010100000001" {
		t.Fatalf("expected subscriber ID '001010100000001', got '%s'", resp.Items[0].SubscriberID)
	}

	if resp.Items[0].SourceIP != "10.0.0.1" {
		t.Fatalf("expected source IP '10.0.0.1', got '%s'", resp.Items[0].SourceIP)
	}

	if resp.Items[0].DestinationIP != "8.8.8.8" {
		t.Fatalf("expected destination IP '8.8.8.8', got '%s'", resp.Items[0].DestinationIP)
	}

	if resp.Items[0].SourcePort != 12345 {
		t.Fatalf("expected source port 12345, got %d", resp.Items[0].SourcePort)
	}

	if resp.Items[0].DestinationPort != 53 {
		t.Fatalf("expected destination port 53, got %d", resp.Items[0].DestinationPort)
	}

	if resp.Items[0].Protocol != 17 {
		t.Fatalf("expected protocol 17, got %d", resp.Items[0].Protocol)
	}

	if resp.Items[0].Packets != 100 {
		t.Fatalf("expected packets 100, got %d", resp.Items[0].Packets)
	}

	if resp.Items[0].Bytes != 50000 {
		t.Fatalf("expected bytes 50000, got %d", resp.Items[0].Bytes)
	}

	if resp.Items[0].StartTime != "2026-02-20T10:00:00Z" {
		t.Fatalf("expected start time '2026-02-20T10:00:00Z', got '%s'", resp.Items[0].StartTime)
	}

	if resp.Items[0].EndTime != "2026-02-20T10:05:00Z" {
		t.Fatalf("expected end time '2026-02-20T10:05:00Z', got '%s'", resp.Items[0].EndTime)
	}

	if resp.Page != 1 {
		t.Fatalf("expected page 1, got %d", resp.Page)
	}

	if resp.PerPage != 25 {
		t.Fatalf("expected per_page 25, got %d", resp.PerPage)
	}

	if resp.TotalCount != 1 {
		t.Fatalf("expected total_count 1, got %d", resp.TotalCount)
	}
}

func TestListFlowReports_Failure(t *testing.T) {
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

	params := &client.ListFlowReportsParams{
		Page:    1,
		PerPage: 25,
	}

	_, err := clientObj.ListFlowReports(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListFlowReportsByDay_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"2026-02-20": [{"id": 1, "subscriber_id": "001010100000001", "source_ip": "10.0.0.1", "destination_ip": "8.8.8.8", "source_port": 12345, "destination_port": 53, "protocol": 17, "packets": 100, "bytes": 50000, "start_time": "2026-02-20T10:00:00Z", "end_time": "2026-02-20T10:05:00Z"}]}, {"2026-02-21": [{"id": 2, "subscriber_id": "001010100000001", "source_ip": "10.0.0.1", "destination_ip": "1.1.1.1", "source_port": 54321, "destination_port": 443, "protocol": 6, "packets": 200, "bytes": 100000, "start_time": "2026-02-21T14:00:00Z", "end_time": "2026-02-21T14:10:00Z"}]}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListFlowReportsParams{
		Start: "2026-02-20",
		End:   "2026-02-21",
	}

	resp, err := clientObj.ListFlowReportsByDay(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(*resp) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(*resp))
	}

	day1Reports := (*resp)[0]["2026-02-20"]
	if len(day1Reports) != 1 {
		t.Fatalf("expected 1 report for 2026-02-20, got %d", len(day1Reports))
	}

	if day1Reports[0].ID != 1 {
		t.Fatalf("expected flow report ID 1, got %d", day1Reports[0].ID)
	}

	if day1Reports[0].Protocol != 17 {
		t.Fatalf("expected protocol 17, got %d", day1Reports[0].Protocol)
	}

	day2Reports := (*resp)[1]["2026-02-21"]
	if len(day2Reports) != 1 {
		t.Fatalf("expected 1 report for 2026-02-21, got %d", len(day2Reports))
	}

	if day2Reports[0].ID != 2 {
		t.Fatalf("expected flow report ID 2, got %d", day2Reports[0].ID)
	}

	if day2Reports[0].Protocol != 6 {
		t.Fatalf("expected protocol 6, got %d", day2Reports[0].Protocol)
	}
}

func TestListFlowReportsByDay_Failure(t *testing.T) {
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

	params := &client.ListFlowReportsParams{
		Start: "2026-02-20",
		End:   "2026-02-21",
	}

	_, err := clientObj.ListFlowReportsByDay(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListFlowReportsBySubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"001010100000001": [{"id": 1, "subscriber_id": "001010100000001", "source_ip": "10.0.0.1", "destination_ip": "8.8.8.8", "source_port": 12345, "destination_port": 53, "protocol": 17, "packets": 100, "bytes": 50000, "start_time": "2026-02-20T10:00:00Z", "end_time": "2026-02-20T10:05:00Z"}]}, {"001010100000002": [{"id": 2, "subscriber_id": "001010100000002", "source_ip": "10.0.0.2", "destination_ip": "1.1.1.1", "source_port": 54321, "destination_port": 443, "protocol": 6, "packets": 200, "bytes": 100000, "start_time": "2026-02-21T14:00:00Z", "end_time": "2026-02-21T14:10:00Z"}]}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListFlowReportsParams{
		Start: "2026-02-20",
		End:   "2026-02-21",
	}

	resp, err := clientObj.ListFlowReportsBySubscriber(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(*resp) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(*resp))
	}

	sub1Reports := (*resp)[0]["001010100000001"]
	if len(sub1Reports) != 1 {
		t.Fatalf("expected 1 report for subscriber 001010100000001, got %d", len(sub1Reports))
	}

	if sub1Reports[0].SubscriberID != "001010100000001" {
		t.Fatalf("expected subscriber ID '001010100000001', got '%s'", sub1Reports[0].SubscriberID)
	}

	sub2Reports := (*resp)[1]["001010100000002"]
	if len(sub2Reports) != 1 {
		t.Fatalf("expected 1 report for subscriber 001010100000002, got %d", len(sub2Reports))
	}

	if sub2Reports[0].SubscriberID != "001010100000002" {
		t.Fatalf("expected subscriber ID '001010100000002', got '%s'", sub2Reports[0].SubscriberID)
	}
}

func TestListFlowReportsBySubscriber_Failure(t *testing.T) {
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

	params := &client.ListFlowReportsParams{
		Start: "2026-02-20",
		End:   "2026-02-21",
	}

	_, err := clientObj.ListFlowReportsBySubscriber(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestClearFlowReports_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "All flow reports cleared successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.ClearFlowReports(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestClearFlowReports_Failure(t *testing.T) {
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

	err := clientObj.ClearFlowReports(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetFlowReportsRetentionPolicy_Success(t *testing.T) {
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

	policy, err := clientObj.GetFlowReportsRetentionPolicy(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Days != 30 {
		t.Fatalf("expected retention days 30, got %d", policy.Days)
	}
}

func TestGetFlowReportsRetentionPolicy_Failure(t *testing.T) {
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

	_, err := clientObj.GetFlowReportsRetentionPolicy(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateFlowReportsRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Flow reports retention policy updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOpts := &client.UpdateFlowReportsRetentionPolicyOptions{
		Days: 60,
	}

	ctx := context.Background()

	err := clientObj.UpdateFlowReportsRetentionPolicy(ctx, updateOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateFlowReportsRetentionPolicy_Failure(t *testing.T) {
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

	updateOpts := &client.UpdateFlowReportsRetentionPolicyOptions{
		Days: -10,
	}

	ctx := context.Background()

	err := clientObj.UpdateFlowReportsRetentionPolicy(ctx, updateOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
