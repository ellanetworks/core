package client_test

import (
	"context"
	"encoding/json"
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
			Result:     []byte(`{"items": [{"name": "radio1"}, {"name": "radio2"}], "page": 1, "per_page": 10, "total_count": 2}`),
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

	resp, err := clientObj.ListRadios(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 radios, got %d", len(resp.Items))
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

	params := &client.ListParams{
		Page:    1,
		PerPage: 10,
	}

	_, err := clientObj.ListRadios(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListRadioEvents_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"id": 1, "timestamp": "2023-10-01T12:00:00Z", "level": "info", "protocol": "ngap", "message_type": "PDU Session Establishment Request", "direction": "inbound", "raw": "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA", "details": "{\"pduSessionID\":1}"}], "page": 1, "per_page": 10, "total_count": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListRadioEventsParams{
		Page:    1,
		PerPage: 10,
	}

	resp, err := clientObj.ListRadioEvents(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 radio event, got %d", len(resp.Items))
	}

	if resp.Items[0].ID != 1 {
		t.Fatalf("expected radio event ID 1, got %d", resp.Items[0].ID)
	}

	if resp.Items[0].Timestamp != "2023-10-01T12:00:00Z" {
		t.Fatalf("expected timestamp '2023-10-01T12:00:00Z', got '%s'", resp.Items[0].Timestamp)
	}

	if resp.Items[0].Protocol != "ngap" {
		t.Fatalf("expected protocol 'ngap', got '%s'", resp.Items[0].Protocol)
	}

	if resp.Items[0].Level != "info" {
		t.Fatalf("expected level 'info', got '%s'", resp.Items[0].Level)
	}

	if resp.Items[0].MessageType != "PDU Session Establishment Request" {
		t.Fatalf("expected message type 'PDU Session Establishment Request', got '%s'", resp.Items[0].MessageType)
	}

	if resp.Items[0].Details != "{\"pduSessionID\":1}" {
		t.Fatalf("expected details '{\"pduSessionID\":1}', got '%s'", resp.Items[0].Details)
	}

	if resp.Items[0].Direction != "inbound" {
		t.Fatalf("expected direction 'inbound', got '%s'", resp.Items[0].Direction)
	}

	expectedRaw := "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA"
	if resp.Items[0].Raw != expectedRaw {
		t.Fatalf("expected raw '%s', got '%s'", expectedRaw, resp.Items[0].Raw)
	}
}

func TestListRadioEvents_Failure(t *testing.T) {
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

	params := &client.ListRadioEventsParams{
		Page:    1,
		PerPage: 10,
	}

	_, err := clientObj.ListRadioEvents(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetRadioEventsRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"days": 15}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	policy, err := clientObj.GetRadioEventRetentionPolicy(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if policy.Days != 15 {
		t.Fatalf("expected retention days 15, got %d", policy.Days)
	}
}

func TestGetRadioEventsRetentionPolicy_Failure(t *testing.T) {
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

	_, err := clientObj.GetRadioEventRetentionPolicy(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateRadioEventsRetentionPolicy_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Radio event retention policy updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOpts := &client.UpdateRadioEventsRetentionPolicyOptions{
		Days: 45,
	}

	ctx := context.Background()

	err := clientObj.UpdateRadioEventRetentionPolicy(ctx, updateOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateRadioEventsRetentionPolicy_Failure(t *testing.T) {
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

	updateOpts := &client.UpdateRadioEventsRetentionPolicyOptions{
		Days: 0,
	}

	ctx := context.Background()

	err := clientObj.UpdateRadioEventRetentionPolicy(ctx, updateOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetRadioEvent_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"raw": "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA", "decoded": {"pduSessionID":1}}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	logID := 1

	resp, err := clientObj.GetRadioEvent(ctx, logID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedRaw := "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA"
	if resp.Raw != expectedRaw {
		t.Fatalf("expected raw '%s', got '%s'", expectedRaw, resp.Raw)
	}

	decodedMap, ok := resp.Decoded.(map[string]any)
	if !ok {
		t.Fatalf("expected decoded to be a map, got %T", resp.Decoded)
	}

	if decodedMap["pduSessionID"] != json.Number("1") { // JSON numbers are float64
		t.Fatalf("expected decoded pduSessionID to be 1, got %v", decodedMap["pduSessionID"])
	}
}

func TestGetRadioEvent_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Radio event not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	logID := 999

	_, err := clientObj.GetRadioEvent(ctx, logID)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
