package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListNetworkInterfaces_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result: []byte(`{
				"n2": {"addresses": ["10.0.0.2"], "port": 38412, "interface": "eth0"},
				"n3": {"name": "eth1", "addresses": ["10.0.0.3"], "external_address": "1.2.3.4"},
				"n6": {"name": "eth2", "addresses": ["10.0.0.6"]},
				"api": {"addresses": ["10.0.0.1"], "port": 5002}
			}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	ifs, err := clientObj.ListNetworkInterfaces(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if ifs.N3.ExternalAddress != "1.2.3.4" {
		t.Errorf("expected N3 external address 1.2.3.4, got: %s", ifs.N3.ExternalAddress)
	}

	if ifs.API.Port != 5002 {
		t.Errorf("expected API port 5002, got: %d", ifs.API.Port)
	}
}

func TestListNetworkInterfaces_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Failed to get N3 settings"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.ListNetworkInterfaces(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateN3Interface_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "N3 interface updated"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateN3InterfaceOptions{
		ExternalAddress: "1.2.3.4",
	}

	ctx := context.Background()

	err := clientObj.UpdateN3Interface(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateN3Interface_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid external address. Must be a valid IP address"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateN3InterfaceOptions{
		ExternalAddress: "not-an-ip",
	}

	ctx := context.Background()

	err := clientObj.UpdateN3Interface(ctx, opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
