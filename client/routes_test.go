package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreateRoute_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Route created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	createRouteOpts := &client.CreateRouteOptions{
		Destination: "1.2.3.4",
		Gateway:     "1.2.3.1",
		Interface:   "eth0",
		Metric:      100,
	}

	ctx := context.Background()

	err := clientObj.CreateRoute(ctx, createRouteOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateRoute_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid Destination"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	createRouteOpts := &client.CreateRouteOptions{
		Destination: "invalid_destination",
		Gateway:     "1.2.3.1",
		Interface:   "eth0",
		Metric:      100,
	}

	ctx := context.Background()

	err := clientObj.CreateRoute(ctx, createRouteOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetRoute_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"id": 123, "destination": "1.2.3.4"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	var id int64 = 123

	getRouteOpts := &client.GetRouteOptions{
		ID: id,
	}

	ctx := context.Background()

	route, err := clientObj.GetRoute(ctx, getRouteOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if route.ID != id {
		t.Fatalf("expected ID %d, got %d", id, route.ID)
	}
}

func TestGetRoute_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Route not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	var id int64 = 123

	getRouteOpts := &client.GetRouteOptions{
		ID: id,
	}

	ctx := context.Background()

	_, err := clientObj.GetRoute(ctx, getRouteOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestDeleteRoute_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Route deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	var id int64 = 132

	deleteRouteOpts := &client.DeleteRouteOptions{
		ID: id,
	}

	ctx := context.Background()

	err := clientObj.DeleteRoute(ctx, deleteRouteOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteRoute_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Route not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	var id int64 = 123

	deleteRouteOpts := &client.DeleteRouteOptions{
		ID: id,
	}

	ctx := context.Background()

	err := clientObj.DeleteRoute(ctx, deleteRouteOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListRoutes_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"id": 1, "destination": "1.2.3.4", "gateway": "1.2.3.1", "interface": "eth0", "metric": 100}], "page": 1, "per_page": 10, "total_count": 1}`),
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

	routes, err := clientObj.ListRoutes(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(routes.Items) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes.Items))
	}
}

func TestListRoutes_Failure(t *testing.T) {
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

	_, err := clientObj.ListRoutes(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
