package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListUsage_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"2022-01-02": {"uplink_bytes": 1000, "downlink_bytes": 2000, "total_bytes": 3000}}, {"2022-01-03": {"uplink_bytes": 1500, "downlink_bytes": 2500, "total_bytes": 4000}}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListUsageParams{
		Start:      "2023-10-01",
		End:        "2023-10-02",
		GroupBy:    "day",
		Subscriber: "",
	}

	resp, err := clientObj.ListUsage(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(*resp) != 2 {
		t.Fatalf("expected 2 usage records, got %d", len(*resp))
	}

	if (*resp)[0]["2022-01-02"].UplinkBytes != 1000 {
		t.Fatalf("expected uplink bytes 1000, got %d", (*resp)[0]["2022-01-02"].UplinkBytes)
	}

	if (*resp)[0]["2022-01-02"].DownlinkBytes != 2000 {
		t.Fatalf("expected downlink bytes 2000, got %d", (*resp)[0]["2022-01-02"].DownlinkBytes)
	}

	if (*resp)[0]["2022-01-02"].TotalBytes != 3000 {
		t.Fatalf("expected total bytes 3000, got %d", (*resp)[0]["2022-01-02"].TotalBytes)
	}

	if (*resp)[1]["2022-01-03"].UplinkBytes != 1500 {
		t.Fatalf("expected uplink bytes 1500, got %d", (*resp)[1]["2022-01-03"].UplinkBytes)
	}

	if (*resp)[1]["2022-01-03"].DownlinkBytes != 2500 {
		t.Fatalf("expected downlink bytes 2500, got %d", (*resp)[1]["2022-01-03"].DownlinkBytes)
	}

	if (*resp)[1]["2022-01-03"].TotalBytes != 4000 {
		t.Fatalf("expected total bytes 4000, got %d", (*resp)[1]["2022-01-03"].TotalBytes)
	}

}

func TestListUsage_Failure(t *testing.T) {
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

	params := &client.ListUsageParams{
		Start:      "2023-10-01",
		End:        "2023-10-02",
		GroupBy:    "day",
		Subscriber: "",
	}

	_, err := clientObj.ListUsage(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
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

func TestGetUsageRetentionPolicy_Failure(t *testing.T) {
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

	_, err := clientObj.GetUsageRetentionPolicy(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
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

func TestUpdateUsageRetentionPolicy_Failure(t *testing.T) {
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

	updateOpts := &client.UpdateUsageRetentionPolicyOptions{
		Days: -10,
	}

	ctx := context.Background()

	err := clientObj.UpdateUsageRetentionPolicy(ctx, updateOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
