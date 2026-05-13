package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetFlowAccountingInfo_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"enabled": true}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	info, err := clientObj.GetFlowAccountingInfo(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if info.Enabled != true {
		t.Errorf("expected flow accounting enabled to be true, got: %v", info.Enabled)
	}
}

func TestGetFlowAccountingInfo_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Flow accounting info not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetFlowAccountingInfo(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateFlowAccountingInfo_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Flow accounting settings updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateFlowAccountingInfoOptions{
		Enabled: true,
	}

	ctx := context.Background()

	err := clientObj.UpdateFlowAccountingInfo(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateFlowAccountingInfo_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid request data"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateFlowAccountingInfoOptions{
		Enabled: false,
	}

	ctx := context.Background()

	err := clientObj.UpdateFlowAccountingInfo(ctx, opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
