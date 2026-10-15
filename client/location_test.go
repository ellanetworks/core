// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetLocation_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"positioningDataList":[{"method":"CELLID"}]}`)},
	}
	clientObj := &client.Client{Requester: fake}

	data, err := clientObj.GetLocation(context.Background(), &client.GetLocationOptions{SUPI: "imsi-001010000000001", Method: "cell_id"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(string(data), "CELLID") {
		t.Fatalf("unexpected location data: %s", data)
	}

	body, _ := io.ReadAll(fake.lastOpts.Body)

	if fake.lastOpts.Method != "POST" || fake.lastOpts.Path != "api/beta/location" || !strings.Contains(string(body), `"method":"cell_id"`) {
		t.Fatalf("unexpected request: %s %s %s", fake.lastOpts.Method, fake.lastOpts.Path, body)
	}
}

func TestCreateDeferredLocationRequest_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{StatusCode: 201, Headers: http.Header{}, Result: []byte(`{"id":"r-1","supi":"imsi-001010000000001","ldr_type":"PERIODIC","periodic_event_info":{"reporting_amount":10,"reporting_interval":30},"method":"cell_id","status":"ACTIVE","event_report_counter":0,"created_at":"2026-10-15T00:00:00Z"}`)},
	}
	clientObj := &client.Client{Requester: fake}

	created, err := clientObj.CreateDeferredLocationRequest(context.Background(), &client.CreateDeferredLocationRequestOptions{
		SUPI:              "imsi-001010000000001",
		LdrType:           "PERIODIC",
		PeriodicEventInfo: client.PeriodicEventInfo{ReportingAmount: 10, ReportingInterval: 30},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if created.ID != "r-1" || created.Status != "ACTIVE" || created.PeriodicEventInfo.ReportingInterval != 30 {
		t.Fatalf("unexpected request: %+v", created)
	}

	body, _ := io.ReadAll(fake.lastOpts.Body)

	if fake.lastOpts.Method != "POST" || fake.lastOpts.Path != "api/beta/location/deferred-requests" || !strings.Contains(string(body), `"ldr_type":"PERIODIC"`) {
		t.Fatalf("unexpected request: %s %s %s", fake.lastOpts.Method, fake.lastOpts.Path, body)
	}
}

func TestListDeferredLocationRequests_Query(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"items":[],"page":2,"per_page":10,"total_count":0}`)},
	}
	clientObj := &client.Client{Requester: fake}

	if _, err := clientObj.ListDeferredLocationRequests(context.Background(), &client.ListDeferredLocationRequestsParams{Page: 2, PerPage: 10, SUPI: "imsi-001010000000001", Status: "ACTIVE"}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	q := fake.lastOpts.Query
	if fake.lastOpts.Path != "api/beta/location/deferred-requests" || q.Get("page") != "2" || q.Get("per_page") != "10" || q.Get("supi") != "imsi-001010000000001" || q.Get("status") != "ACTIVE" {
		t.Fatalf("unexpected request: %s %v", fake.lastOpts.Path, q)
	}
}

func TestDeleteDeferredLocationRequest_Path(t *testing.T) {
	fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 204, Headers: http.Header{}}}
	clientObj := &client.Client{Requester: fake}

	if err := clientObj.DeleteDeferredLocationRequest(context.Background(), "r-1"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/beta/location/deferred-requests/r-1" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	if err := clientObj.DeleteDeferredLocationRequest(context.Background(), ""); err == nil {
		t.Fatal("expected an error for an empty id")
	}
}

func TestListLocationReports_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"items":[{"seq":4,"reported_at":"2026-10-15T00:00:00Z","location_data":{"ncgi":{}}}],"next":4}`)},
	}
	clientObj := &client.Client{Requester: fake}

	reports, err := clientObj.ListLocationReports(context.Background(), "r-1", 3, 50)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(reports.Items) != 1 || reports.Items[0].Seq != 4 || reports.Next != 4 || !strings.Contains(string(reports.Items[0].LocationData), "ncgi") {
		t.Fatalf("unexpected reports: %+v", reports)
	}

	q := fake.lastOpts.Query
	if fake.lastOpts.Path != "api/beta/location/deferred-requests/r-1/reports" || q.Get("after") != "3" || q.Get("limit") != "50" {
		t.Fatalf("unexpected request: %s %v", fake.lastOpts.Path, q)
	}
}
