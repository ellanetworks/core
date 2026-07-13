// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestListCellPositions_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"id": "cp-1", "rat": "nr", "mcc": "001", "mnc": "01", "cell_identity": "00000001", "latitude": 45.5, "longitude": -73.6, "source": "provisioned"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	resp, err := clientObj.ListCellPositions(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp) != 1 {
		t.Fatalf("expected 1 cell position, got %d", len(resp))
	}

	if resp[0].ID != "cp-1" || resp[0].RAT != "nr" || resp[0].CellIdentity != "00000001" {
		t.Fatalf("unexpected item: %+v", resp[0])
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/beta/cell-positions" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestGetCellPosition_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"id": "cp-1", "rat": "eutra", "mcc": "001", "mnc": "01", "cell_identity": "0000001", "latitude": 45.5, "longitude": -73.6, "source": "provisioned"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	resp, err := clientObj.GetCellPosition(context.Background(), "cp-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.ID != "cp-1" || resp.RAT != "eutra" {
		t.Fatalf("unexpected cell position: %+v", resp)
	}

	if fake.lastOpts.Path != "api/beta/cell-positions/cp-1" {
		t.Fatalf("unexpected path: %s", fake.lastOpts.Path)
	}
}

func TestGetCellPosition_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Cell position not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{Requester: fake}

	if _, err := clientObj.GetCellPosition(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestCreateCellPosition_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Cell position created", "id": "cp-1"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	err := clientObj.CreateCellPosition(context.Background(), &client.CreateCellPositionOptions{
		RAT:          "nr",
		Mcc:          "001",
		Mnc:          "01",
		CellIdentity: "00000001",
		Latitude:     45.5,
		Longitude:    -73.6,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "POST" || fake.lastOpts.Path != "api/beta/cell-positions" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestCreateCellPosition_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "mcc and mnc are required"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{Requester: fake}

	err := clientObj.CreateCellPosition(context.Background(), &client.CreateCellPositionOptions{RAT: "nr"})
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateCellPosition_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Cell position updated"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	err := clientObj.UpdateCellPosition(context.Background(), "cp-1", &client.UpdateCellPositionOptions{
		RAT:          "nr",
		Mcc:          "001",
		Mnc:          "01",
		CellIdentity: "00000001",
		Latitude:     46.0,
		Longitude:    -73.0,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" || fake.lastOpts.Path != "api/beta/cell-positions/cp-1" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestDeleteCellPosition_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Cell position deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{Requester: fake}

	if err := clientObj.DeleteCellPosition(context.Background(), "cp-1"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/beta/cell-positions/cp-1" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestDeleteCellPosition_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Cell position not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{Requester: fake}

	if err := clientObj.DeleteCellPosition(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error, got none")
	}
}
