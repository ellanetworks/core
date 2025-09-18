package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetRadio_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "my-radio"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	name := "my-radio"

	getRouteOpts := &client.GetRadioOptions{
		Name: name,
	}

	ctx := context.Background()

	radio, err := clientObj.GetRadio(ctx, getRouteOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if radio.Name != name {
		t.Fatalf("expected ID %v, got %v", name, radio.Name)
	}
}

func TestGetRadio_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Radio not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	name := "non-existent-radio"
	getRadioOpts := &client.GetRadioOptions{
		Name: name,
	}

	ctx := context.Background()

	_, err := clientObj.GetRadio(ctx, getRadioOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListRadios_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"name": "my-name"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	radios, err := clientObj.ListRadios(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(radios) != 1 {
		t.Fatalf("expected 1 radio, got %d", len(radios))
	}
}

func TestListRadios_Failure(t *testing.T) {
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

	_, err := clientObj.ListRadios(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
