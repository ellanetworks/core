package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetOperator_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"id": {"mcc": "001", "mnc": "01"}}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	operator, err := clientObj.GetOperator(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if operator.ID.Mcc != "001" {
		t.Fatalf("expected ID %v, got %v", "001", operator.ID.Mcc)
	}
	if operator.ID.Mnc != "01" {
		t.Fatalf("expected ID %v, got %v", "01", operator.ID.Mnc)
	}
}

func TestGetOperator_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Operator not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetOperator(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateOperatorID_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Operator ID updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOperatorIDOpts := &client.UpdateOperatorIDOptions{
		Mcc: "001",
		Mnc: "01",
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorID(ctx, updateOperatorIDOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateOperatorID_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid UE IP Pool"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	updateOperatorIDOpts := &client.UpdateOperatorIDOptions{
		Mcc: "001",
		Mnc: "01",
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorID(ctx, updateOperatorIDOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateOperatorSlice_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Operator Slice updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOperatorSliceOpts := &client.UpdateOperatorSliceOptions{
		Sst: 1,
		Sd:  "0x012030",
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorSlice(ctx, updateOperatorSliceOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateOperatorSlice_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid SSD"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	updateOperatorSliceOpts := &client.UpdateOperatorSliceOptions{
		Sst: 1,
		Sd:  "0x012030",
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorSlice(ctx, updateOperatorSliceOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateOperatorTracking_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Operator Tracking updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOperatorTrackingOpts := &client.UpdateOperatorTrackingOptions{
		SupportedTacs: []string{"001", "002"},
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorTracking(ctx, updateOperatorTrackingOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateOperatorTracking_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid tac"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	updateOperatorTrackingOpts := &client.UpdateOperatorTrackingOptions{
		SupportedTacs: []string{"001", "002"},
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorTracking(ctx, updateOperatorTrackingOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
