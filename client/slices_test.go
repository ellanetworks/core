package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreateSlice_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Slice created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.CreateSliceOptions{
		Name: "default",
		Sst:  1,
		Sd:   "000001",
	}

	ctx := context.Background()

	err := clientObj.CreateSlice(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Fatalf("expected POST method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/slices" {
		t.Fatalf("expected path api/v1/slices, got: %s", fake.lastOpts.Path)
	}
}

func TestCreateSlice_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 409,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "A network slice already exists"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.CreateSliceOptions{
		Name: "second",
		Sst:  2,
	}

	ctx := context.Background()

	err := clientObj.CreateSlice(ctx, opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetSlice_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "default", "sst": 1, "sd": "000001"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	slice, err := clientObj.GetSlice(ctx, &client.GetSliceOptions{Name: "default"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if slice.Name != "default" {
		t.Fatalf("expected name 'default', got: %s", slice.Name)
	}

	if slice.Sst != 1 {
		t.Fatalf("expected sst 1, got: %d", slice.Sst)
	}

	if slice.Sd != "000001" {
		t.Fatalf("expected sd '000001', got: %s", slice.Sd)
	}
}

func TestGetSlice_WithoutSd(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "default", "sst": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	slice, err := clientObj.GetSlice(ctx, &client.GetSliceOptions{Name: "default"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if slice.Sd != "" {
		t.Fatalf("expected empty sd, got: %s", slice.Sd)
	}
}

func TestGetSlice_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Slice not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetSlice(ctx, &client.GetSliceOptions{Name: "non-existent"})
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateSlice_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Slice updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateSliceOptions{
		Sst: 1,
		Sd:  "000002",
	}

	ctx := context.Background()

	err := clientObj.UpdateSlice(ctx, "default", opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected PUT method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/slices/default" {
		t.Fatalf("expected path api/v1/slices/default, got: %s", fake.lastOpts.Path)
	}
}

func TestUpdateSlice_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Slice not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateSliceOptions{
		Sst: 2,
	}

	ctx := context.Background()

	err := clientObj.UpdateSlice(ctx, "non-existent", opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestDeleteSlice_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Slice deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: "default"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" {
		t.Fatalf("expected DELETE method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/slices/default" {
		t.Fatalf("expected path api/v1/slices/default, got: %s", fake.lastOpts.Path)
	}
}

func TestDeleteSlice_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 409,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Slice has policies"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteSlice(ctx, &client.DeleteSliceOptions{Name: "default"})
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListSlices_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"name": "default", "sst": 1, "sd": "000001"}], "page": 1, "per_page": 10, "total_count": 1}`),
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

	resp, err := clientObj.ListSlices(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 slice, got: %d", len(resp.Items))
	}

	if resp.Items[0].Name != "default" {
		t.Fatalf("expected slice name 'default', got: %s", resp.Items[0].Name)
	}

	if resp.Items[0].Sst != 1 {
		t.Fatalf("expected sst 1, got: %d", resp.Items[0].Sst)
	}
}

func TestListSlices_Failure(t *testing.T) {
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

	_, err := clientObj.ListSlices(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
