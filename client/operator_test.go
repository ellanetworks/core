// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
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
		Ciphering: []string{"AES", "SNOW3G"},
		Integrity: []string{"AES", "SNOW3G"},
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

func TestCreateHomeNetworkKey_Success(t *testing.T) {
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

	err := clientObj.DeleteHomeNetworkKey(ctx, "0190b3d2-7c12-7c00-8000-000000000001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" {
		t.Fatalf("expected method DELETE, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/operator/home-network-keys/0190b3d2-7c12-7c00-8000-000000000001" {
		t.Fatalf("expected path api/v1/operator/home-network-keys/0190b3d2-7c12-7c00-8000-000000000001, got %s", fake.lastOpts.Path)
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

	resp, err := clientObj.GetHomeNetworkKeyPrivateKey(ctx, "0190b3d2-7c12-7c00-8000-000000000007")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if resp.PrivateKey != "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789" {
		t.Fatalf("unexpected private key: %s", resp.PrivateKey)
	}

	if fake.lastOpts.Method != "GET" {
		t.Fatalf("expected method GET, got %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/operator/home-network-keys/0190b3d2-7c12-7c00-8000-000000000007/private-key" {
		t.Fatalf("expected path api/v1/operator/home-network-keys/0190b3d2-7c12-7c00-8000-000000000007/private-key, got %s", fake.lastOpts.Path)
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
					{"id": "018f0000-0000-7000-8000-000000000001", "keyIdentifier": 0, "scheme": "A", "publicKey": "aabb"},
					{"id": "018f0000-0000-7000-8000-000000000002", "keyIdentifier": 0, "scheme": "B", "publicKey": "02cc"}
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

func TestUpdateOperatorCode_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Operator Code updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.UpdateOperatorCode(ctx, &client.UpdateOperatorCodeOptions{
		OperatorCode: "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected PUT method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/operator/code" {
		t.Fatalf("expected path api/v1/operator/code, got: %s", fake.lastOpts.Path)
	}
}

func TestGetOperator_IncludesSPN(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result: []byte(`{
				"id": {"mcc": "001", "mnc": "01"},
				"spn": {"fullName": "My Network", "shortName": "MyNet"}
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

	if operator.SPN.FullName != "My Network" {
		t.Fatalf("expected fullName 'My Network', got '%s'", operator.SPN.FullName)
	}

	if operator.SPN.ShortName != "MyNet" {
		t.Fatalf("expected shortName 'MyNet', got '%s'", operator.SPN.ShortName)
	}
}

func TestUpdateOperatorSMS_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Operator SMS settings updated successfully"}`),
		},
	}
	clientObj := &client.Client{Requester: fake}

	err := clientObj.UpdateOperatorSMS(context.Background(), &client.UpdateOperatorSMSOptions{
		Enabled:   true,
		SMSNumber: "+15550001111",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" || fake.lastOpts.Path != "api/v1/operator/sms" {
		t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	var payload map[string]any
	if err := json.NewDecoder(fake.lastOpts.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if len(payload) != 2 || payload["enabled"] != true || payload["smsNumber"] != "+15550001111" {
		t.Fatalf("unexpected payload %v", payload)
	}
}

func TestSMSCPeerRequests(t *testing.T) {
	opts := &client.SMSCPeerOptions{Address: "192.0.2.10", Port: 3869, DiameterIdentity: "smsc.example.org", ServiceCentres: []string{"+15550000000"}}

	cases := []struct {
		name   string
		call   func(c *client.Client) error
		method string
		path   string
		body   bool
	}{
		{"create", func(c *client.Client) error {
			_, err := c.CreateSMSCPeer(context.Background(), opts)
			return err
		}, "POST", "api/v1/operator/sms/smsc-peers", true},
		{"update", func(c *client.Client) error { return c.UpdateSMSCPeer(context.Background(), "abc", opts) }, "PUT", "api/v1/operator/sms/smsc-peers/abc", true},
		{"delete", func(c *client.Client) error { return c.DeleteSMSCPeer(context.Background(), "abc") }, "DELETE", "api/v1/operator/sms/smsc-peers/abc", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"message": "ok"}`)}}

			if err := tc.call(&client.Client{Requester: fake}); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if fake.lastOpts.Method != tc.method || fake.lastOpts.Path != tc.path {
				t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
			}

			if !tc.body {
				return
			}

			var payload struct {
				Address          string   `json:"address"`
				Port             int      `json:"port"`
				DiameterIdentity string   `json:"diameterIdentity"`
				ServiceCentres   []string `json:"serviceCentres"`
			}

			if err := json.NewDecoder(fake.lastOpts.Body).Decode(&payload); err != nil {
				t.Fatalf("decode body: %v", err)
			}

			if payload.Address != "192.0.2.10" || payload.Port != 3869 || payload.DiameterIdentity != "smsc.example.org" || !reflect.DeepEqual(payload.ServiceCentres, []string{"+15550000000"}) {
				t.Fatalf("unexpected payload %+v", payload)
			}
		})
	}
}

func TestGetOperator_IncludesSMS(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"sms": {"enabled": true, "smsNumber": "+15550001111"}}`),
		},
	}
	clientObj := &client.Client{Requester: fake}

	operator, err := clientObj.GetOperator(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := client.GetOperatorSMSResponse{Enabled: true, SMSNumber: "+15550001111"}
	if operator.SMS != want {
		t.Fatalf("sms = %+v, want %+v", operator.SMS, want)
	}
}

func TestSMSCPeerReads(t *testing.T) {
	peer := `{"id": "abc", "address": "192.0.2.10", "port": 3868, "diameterIdentity": "", "serviceCentres": ["+15550000000"],
		"status": {"state": "open", "host": "smsc.example.org", "realm": "example.org", "since": "2026-09-29T16:00:00Z"}}`
	want := client.SMSCPeer{
		ID: "abc", Address: "192.0.2.10", Port: 3868, ServiceCentres: []string{"+15550000000"},
		Status: &client.SMSCPeerStatus{State: "open", Host: "smsc.example.org", Realm: "example.org", Since: "2026-09-29T16:00:00Z"},
	}

	fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"items": [` + peer + `]}`)}}

	list, err := (&client.Client{Requester: fake}).ListSMSCPeers(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/operator/sms/smsc-peers" || len(list.Items) != 1 || !reflect.DeepEqual(list.Items[0], want) {
		t.Fatalf("list = %+v via %s %s", list, fake.lastOpts.Method, fake.lastOpts.Path)
	}

	for name, call := range map[string]func(c *client.Client) (*client.SMSCPeer, error){
		"get": func(c *client.Client) (*client.SMSCPeer, error) { return c.GetSMSCPeer(context.Background(), "abc") },
		"create": func(c *client.Client) (*client.SMSCPeer, error) {
			return c.CreateSMSCPeer(context.Background(), &client.SMSCPeerOptions{Address: "192.0.2.10", ServiceCentres: []string{"+15550000000"}})
		},
	} {
		fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(peer)}}

		got, err := call(&client.Client{Requester: fake})
		if err != nil || !reflect.DeepEqual(*got, want) {
			t.Fatalf("%s = %+v, %v", name, got, err)
		}
	}
}
