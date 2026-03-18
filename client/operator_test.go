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
		Sd:  "012030",
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
		Sd:  "012030",
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

func TestUpdateOperatorNASSecurity_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Operator NAS security algorithms updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	updateOperatorNASSecurityOpts := &client.UpdateOperatorNASSecurityOptions{
		Ciphering: []string{"NEA2", "NEA1"},
		Integrity: []string{"NIA2", "NIA1"},
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorNASSecurity(ctx, updateOperatorNASSecurityOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts == nil {
		t.Fatal("expected request options to be captured")
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected method PUT, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/operator/nas-security" {
		t.Fatalf("expected path api/v1/operator/nas-security, got %s", fake.lastOpts.Path)
	}
}

func TestUpdateOperatorNASSecurity_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid ciphering algorithm"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	updateOperatorNASSecurityOpts := &client.UpdateOperatorNASSecurityOptions{
		Ciphering: []string{"NEA9"},
		Integrity: []string{"NIA2"},
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorNASSecurity(ctx, updateOperatorNASSecurityOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestCreateHomeNetworkKey_ProfileA(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Home network key created"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.CreateHomeNetworkKeyOptions{
		KeyIdentifier: 0,
		Scheme:        "A",
		PrivateKey:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	ctx := context.Background()

	err := clientObj.CreateHomeNetworkKey(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Fatalf("expected method POST, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/operator/home-network-keys" {
		t.Fatalf("expected path api/v1/operator/home-network-keys, got %s", fake.lastOpts.Path)
	}
}

func TestCreateHomeNetworkKey_ProfileB(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Home network key created"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.CreateHomeNetworkKeyOptions{
		KeyIdentifier: 0,
		Scheme:        "B",
		PrivateKey:    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}

	ctx := context.Background()

	err := clientObj.CreateHomeNetworkKey(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "POST" {
		t.Fatalf("expected method POST, got %s", fake.lastOpts.Method)
	}
}

func TestDeleteHomeNetworkKey_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Home network key deleted"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteHomeNetworkKey(ctx, 42)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" {
		t.Fatalf("expected method DELETE, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/operator/home-network-keys/42" {
		t.Fatalf("expected path api/v1/operator/home-network-keys/42, got %s", fake.lastOpts.Path)
	}
}

func TestDeleteHomeNetworkKey_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteHomeNetworkKey(ctx, 999)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetHomeNetworkKeyPrivateKey_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"privateKey": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	resp, err := clientObj.GetHomeNetworkKeyPrivateKey(ctx, 7)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.PrivateKey != "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" {
		t.Fatalf("unexpected private key: %s", resp.PrivateKey)
	}

	if fake.lastOpts.Method != "GET" {
		t.Fatalf("expected method GET, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/operator/home-network-keys/7/private-key" {
		t.Fatalf("expected path api/v1/operator/home-network-keys/7/private-key, got %s", fake.lastOpts.Path)
	}
}

func TestGetHomeNetworkKeyPrivateKey_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetHomeNetworkKeyPrivateKey(ctx, 999)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetOperator_IncludesHomeNetworkKeys(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result: []byte(`{
				"id": {"mcc": "001", "mnc": "01"},
				"homeNetworkKeys": [
					{"id": 1, "keyIdentifier": 0, "scheme": "A", "publicKey": "aabb"},
					{"id": 2, "keyIdentifier": 0, "scheme": "B", "publicKey": "02cc"}
				]
			}`),
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

	if len(operator.HomeNetworkKeys) != 2 {
		t.Fatalf("expected 2 home network keys, got %d", len(operator.HomeNetworkKeys))
	}

	if operator.HomeNetworkKeys[0].Scheme != "A" {
		t.Fatalf("expected first key scheme A, got %s", operator.HomeNetworkKeys[0].Scheme)
	}

	if operator.HomeNetworkKeys[1].Scheme != "B" {
		t.Fatalf("expected second key scheme B, got %s", operator.HomeNetworkKeys[1].Scheme)
	}
}

func TestUpdateOperatorSPN_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Operator SPN updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateOperatorSPNOptions{
		FullName:  "Ella Networks",
		ShortName: "Ella",
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorSPN(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts == nil {
		t.Fatal("expected request options to be captured")
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected method PUT, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/operator/spn" {
		t.Fatalf("expected path api/v1/operator/spn, got %s", fake.lastOpts.Path)
	}
}

func TestUpdateOperatorSPN_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "fullName is required and must not be empty"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateOperatorSPNOptions{
		FullName:  "",
		ShortName: "Ella",
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorSPN(ctx, opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetOperator_IncludesSPN(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result: []byte(`{
				"id": {"mcc": "001", "mnc": "01"},
				"spn": {"spnFull": "My Network", "spnShort": "MyNet"}
			}`),
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

	if operator.SPN.SpnFull != "My Network" {
		t.Fatalf("expected spnFull 'My Network', got '%s'", operator.SPN.SpnFull)
	}

	if operator.SPN.SpnShort != "MyNet" {
		t.Fatalf("expected spnShort 'MyNet', got '%s'", operator.SPN.SpnShort)
	}
}
