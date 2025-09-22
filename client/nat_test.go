package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetNATInfo_Success(t *testing.T) {
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

	natInfo, err := clientObj.GetNATInfo(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if natInfo.Enabled != true {
		t.Errorf("expected NAT enabled to be true, got: %v", natInfo.Enabled)
	}
}

func TestGetNATInfo_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "NAT info not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetNATInfo(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateNATInfo_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "NAT Info updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateNATInfoOpts := &client.UpdateNATInfoOptions{
		Enabled: false,
	}

	ctx := context.Background()

	err := clientObj.UpdateNATInfo(ctx, updateNATInfoOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateNATInfo_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid NAT info"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateNATInfoOpts := &client.UpdateNATInfoOptions{
		Enabled: false,
	}

	ctx := context.Background()

	err := clientObj.UpdateNATInfo(ctx, updateNATInfoOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
