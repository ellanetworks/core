package client_test

import (
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

	err := clientObj.CreateRoute(createRouteOpts)
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

	err := clientObj.CreateRoute(createRouteOpts)
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
	route, err := clientObj.GetRoute(getRouteOpts)
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
	_, err := clientObj.GetRoute(getRouteOpts)
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
	err := clientObj.DeleteRoute(deleteRouteOpts)
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
	err := clientObj.DeleteRoute(deleteRouteOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListRoutes_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"imsi": "001010100000022", "profileName": "default"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	routes, err := clientObj.ListRoutes()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
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

	_, err := clientObj.ListRoutes()
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
