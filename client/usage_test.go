// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListUsagePerDay_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"date": "2022-01-02", "uplink_bytes": 1000, "downlink_bytes": 2000, "total_bytes": 3000}, {"date": "2022-01-03", "uplink_bytes": 1500, "downlink_bytes": 2500, "total_bytes": 4000}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	resp, err := clientObj.ListUsagePerDay(context.Background(), &client.ListUsageParams{
		Start: "2023-10-01T00:00:00Z",
		End:   "2023-10-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Query.Get("group_by") != "day" {
		t.Fatalf("expected group_by=day, got %q", fake.lastOpts.Query.Get("group_by"))
	}

	want := []client.DailySubscriberUsage{
		{Date: "2022-01-02", UplinkBytes: 1000, DownlinkBytes: 2000, TotalBytes: 3000},
		{Date: "2022-01-03", UplinkBytes: 1500, DownlinkBytes: 2500, TotalBytes: 4000},
	}

	if len(resp) != len(want) {
		t.Fatalf("expected %d usage records, got %d", len(want), len(resp))
	}

	for i, w := range want {
		if resp[i] != w {
			t.Errorf("record %d: got %+v, want %+v", i, resp[i], w)
		}
	}
}

func TestListUsagePerSubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"imsi": "001010000000001", "uplink_bytes": 1000, "downlink_bytes": 2000, "total_bytes": 3000}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	resp, err := clientObj.ListUsagePerSubscriber(context.Background(), &client.ListUsageParams{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Query.Get("group_by") != "subscriber" {
		t.Fatalf("expected group_by=subscriber, got %q", fake.lastOpts.Query.Get("group_by"))
	}

	want := client.PerSubscriberUsage{IMSI: "001010000000001", UplinkBytes: 1000, DownlinkBytes: 2000, TotalBytes: 3000}

	if len(resp) != 1 || resp[0] != want {
		t.Fatalf("got %+v, want [%+v]", resp, want)
	}
}

func TestGetUsageRetentionPolicy_Success(t *testing.T) {
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

	policy, err := clientObj.GetUsageRetentionPolicy(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Days != 30 {
		t.Fatalf("expected retention days 30, got %d", policy.Days)
	}
}

func TestUpdateUsageRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Usage retention policy updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOpts := &client.UpdateUsageRetentionPolicyOptions{
		Days: 60,
	}

	ctx := context.Background()

	err := clientObj.UpdateUsageRetentionPolicy(ctx, updateOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestClearUsage_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "All subscriber usage cleared successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	if err := clientObj.ClearUsage(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/v1/subscriber-usage" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}
