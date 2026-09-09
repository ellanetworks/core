// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreateSubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Subscriber created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	createSubscriberOpts := &client.CreateSubscriberOptions{
		Imsi:           "001010100000022",
		Key:            "5122250214c33e723a5dd523fc145fc0",
		SequenceNumber: "000000000022",
		ProfileName:    "default",
	}

	ctx := context.Background()

	err := clientObj.CreateSubscriber(ctx, createSubscriberOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestGetSubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"imsi": "001010100000022", "profile_name": "default", "registrations": [{"system": "5GS", "registered": true, "connection_state": "connected", "radio": "gnb-01", "imei": "359881234567890", "ciphering_algorithm": "128-NEA2", "integrity_algorithm": "128-NIA2", "connection": {"amf_ue_ngap_id": 12, "ran_ue_ngap_id": 39}}], "sessions": [{"system": "5GS", "access_types": ["3GPP"], "id": 1, "status": "active", "ipv4_address": "10.45.0.2", "data_network": "internet", "slice": {"sst": 1, "sd": "000001"}, "ambr_uplink": "100 Mbps", "ambr_downlink": "200 Mbps"}]}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	imsi := "001010100000022"

	getSubOpts := &client.GetSubscriberOptions{
		ID: imsi,
	}

	ctx := context.Background()

	subscriber, err := clientObj.GetSubscriber(ctx, getSubOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if subscriber.Imsi != imsi {
		t.Fatalf("expected IMSI %s, got %s", imsi, subscriber.Imsi)
	}

	if len(subscriber.Registrations) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(subscriber.Registrations))
	}

	reg := subscriber.Registrations[0]

	if reg.System != "5GS" {
		t.Fatalf("expected a 5GS registration, got %s", reg.System)
	}

	if !reg.Registered {
		t.Fatalf("expected Registered true, got %v", reg.Registered)
	}

	if reg.ConnectionState == nil || *reg.ConnectionState != "connected" {
		t.Fatalf("expected ConnectionState 'connected', got %v", reg.ConnectionState)
	}

	if reg.Radio != "gnb-01" {
		t.Fatalf("expected Radio 'gnb-01', got %s", reg.Radio)
	}

	if reg.Imei != "359881234567890" {
		t.Fatalf("expected Imei '359881234567890', got %s", reg.Imei)
	}

	if reg.CipheringAlgorithm != "128-NEA2" || reg.IntegrityAlgorithm != "128-NIA2" {
		t.Fatalf("expected 128-NEA2/128-NIA2, got %s/%s", reg.CipheringAlgorithm, reg.IntegrityAlgorithm)
	}

	if reg.Connection == nil || reg.Connection.AmfUeNgapID == nil || *reg.Connection.AmfUeNgapID != 12 {
		t.Fatalf("expected amf_ue_ngap_id 12, got %+v", reg.Connection)
	}

	if reg.Connection.RanUeNgapID == nil || *reg.Connection.RanUeNgapID != 39 {
		t.Fatalf("expected ran_ue_ngap_id 39, got %+v", reg.Connection.RanUeNgapID)
	}

	if reg.Connection.MMEUeS1apID != nil || reg.Connection.ENBUeS1apID != nil {
		t.Fatalf("expected no S1AP identities on a 5GS registration, got %+v", reg.Connection)
	}

	if len(subscriber.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(subscriber.Sessions))
	}

	if subscriber.Sessions[0].System != "5GS" {
		t.Fatalf("expected session system '5GS', got %s", subscriber.Sessions[0].System)
	}

	if got := subscriber.Sessions[0].AccessTypes; len(got) != 1 || got[0] != "3GPP" {
		t.Fatalf("expected session access_types [3GPP], got %v", got)
	}

	if subscriber.Sessions[0].ID != 1 {
		t.Fatalf("expected session ID 1, got %d", subscriber.Sessions[0].ID)
	}

	if subscriber.Sessions[0].Status != "active" {
		t.Fatalf("expected session status 'active', got %s", subscriber.Sessions[0].Status)
	}

	if subscriber.Sessions[0].IPv4Address != "10.45.0.2" {
		t.Fatalf("expected session IP '10.45.0.2', got %s", subscriber.Sessions[0].IPv4Address)
	}

	if subscriber.Sessions[0].DataNetwork != "internet" {
		t.Fatalf("expected session data network 'internet', got %s", subscriber.Sessions[0].DataNetwork)
	}

	if subscriber.Sessions[0].Slice == nil || subscriber.Sessions[0].Slice.SST != 1 {
		t.Fatalf("expected session slice SST 1, got %v", subscriber.Sessions[0].Slice)
	}

	if subscriber.Sessions[0].Slice.SD != "000001" {
		t.Fatalf("expected session slice SD '000001', got %s", subscriber.Sessions[0].Slice.SD)
	}

	if subscriber.Sessions[0].AMBRUplink != "100 Mbps" {
		t.Fatalf("expected session AMBR up '100 Mbps', got %s", subscriber.Sessions[0].AMBRUplink)
	}

	if subscriber.Sessions[0].AMBRDownlink != "200 Mbps" {
		t.Fatalf("expected session AMBR down '200 Mbps', got %s", subscriber.Sessions[0].AMBRDownlink)
	}
}

func TestUpdateSubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Subscriber updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateSubscriberOptions{
		ProfileName: "enterprise",
	}

	ctx := context.Background()

	err := clientObj.UpdateSubscriber(ctx, "001010100000022", opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" {
		t.Fatalf("expected PUT method, got: %s", fake.lastOpts.Method)
	}

	if fake.lastOpts.Path != "api/v1/subscribers/001010100000022" {
		t.Fatalf("expected path api/v1/subscribers/001010100000022, got: %s", fake.lastOpts.Path)
	}
}

func TestDeleteSubscriber_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Subscriber deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	imsi := "001010100000022"

	deleteSubOpts := &client.DeleteSubscriberOptions{
		ID: imsi,
	}

	ctx := context.Background()

	err := clientObj.DeleteSubscriber(ctx, deleteSubOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestListSubscribers_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"imsi": "001010100000022", "profile_name": "default", "status": {"registered": true, "connection_state": "idle", "num_sessions": 1, "last_seen_at": "2025-01-01T00:00:00Z", "last_seen_radio": "gnb-01"}}], "page": 1, "per_page": 10, "total_count": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListSubscribersParams{
		Page:    1,
		PerPage: 10,
	}

	resp, err := clientObj.ListSubscribers(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(resp.Items))
	}

	sub := resp.Items[0]
	if sub.Status.LastSeenRadio != "gnb-01" {
		t.Fatalf("expected last_seen_radio %q, got %q", "gnb-01", sub.Status.LastSeenRadio)
	}

	if sub.Status.ConnectionState != "idle" {
		t.Fatalf("expected connection_state %q, got %q", "idle", sub.Status.ConnectionState)
	}

	if sub.Status.LastSeenAt != "2025-01-01T00:00:00Z" {
		t.Fatalf("expected lastSeenAt %q, got %q", "2025-01-01T00:00:00Z", sub.Status.LastSeenAt)
	}

	if sub.Status.NumSessions != 1 {
		t.Fatalf("expected num_sessions 1, got %d", sub.Status.NumSessions)
	}
}

func TestListSubscribers_Query(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params client.ListSubscribersParams
		want   string
	}{
		{"no filters", client.ListSubscribersParams{Page: 1, PerPage: 10}, "page=1&per_page=10"},
		{"search", client.ListSubscribersParams{Page: 1, PerPage: 10, Search: "0748"}, "page=1&per_page=10&search=0748"},
		{"radio and search", client.ListSubscribersParams{Page: 2, PerPage: 25, Radio: "gnb-01", Search: "0748"}, "page=2&per_page=25&radio=gnb-01&search=0748"},
		{"search needing escaping", client.ListSubscribersParams{Page: 1, PerPage: 10, Search: "a b&c"}, "page=1&per_page=10&search=a+b%26c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRequester{
				response: &client.RequestResponse{
					StatusCode: 200,
					Headers:    http.Header{},
					Result:     []byte(`{"items": [], "page": 1, "per_page": 10, "total_count": 0}`),
				},
			}
			clientObj := &client.Client{Requester: fake}

			if _, err := clientObj.ListSubscribers(context.Background(), &tc.params); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if got := fake.lastOpts.Query.Encode(); got != tc.want {
				t.Fatalf("expected query %q, got %q", tc.want, got)
			}
		})
	}
}

func TestGetSubscriberCredentials_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"key": "5122250214c33e723a5dd523fc145fc0", "opc": "b9f9d006cbe505a0b79f1ad0b3e44d95", "sequenceNumber": "16f3b3f70fc2"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	opts := &client.GetSubscriberCredentialsOptions{
		ID: "001010100000022",
	}

	creds, err := clientObj.GetSubscriberCredentials(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if creds.Key != "5122250214c33e723a5dd523fc145fc0" {
		t.Fatalf("expected key 5122250214c33e723a5dd523fc145fc0, got %s", creds.Key)
	}

	if creds.Opc != "b9f9d006cbe505a0b79f1ad0b3e44d95" {
		t.Fatalf("expected opc b9f9d006cbe505a0b79f1ad0b3e44d95, got %s", creds.Opc)
	}

	if creds.SequenceNumber != "16f3b3f70fc2" {
		t.Fatalf("expected sequenceNumber 16f3b3f70fc2, got %s", creds.SequenceNumber)
	}
}

func TestSubscriberDescriptionIsMarshalled(t *testing.T) {
	decodeBody := func(t *testing.T, fake *fakeRequester) map[string]string {
		t.Helper()

		if fake.lastOpts == nil {
			t.Fatal("expected RequestOptions to be set, but got nil")
		}

		bodyBytes, err := io.ReadAll(fake.lastOpts.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		var payload map[string]string
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}

		return payload
	}

	t.Run("create sends the description", func(t *testing.T) {
		fake := &fakeRequester{
			response: &client.RequestResponse{
				StatusCode: 200,
				Headers:    http.Header{},
				Result:     []byte(`{"message": "Subscriber created successfully"}`),
			},
		}
		clientObj := &client.Client{Requester: fake}

		err := clientObj.CreateSubscriber(context.Background(), &client.CreateSubscriberOptions{
			Imsi:           "001010100000022",
			Key:            "5122250214c33e723a5dd523fc145fc0",
			SequenceNumber: "000000000022",
			ProfileName:    "default",
			Description:    "Warehouse gate reader",
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if got := decodeBody(t, fake)["description"]; got != "Warehouse gate reader" {
			t.Fatalf("description = %q, want %q", got, "Warehouse gate reader")
		}
	})

	t.Run("update sends the description", func(t *testing.T) {
		fake := &fakeRequester{
			response: &client.RequestResponse{
				StatusCode: 200,
				Headers:    http.Header{},
				Result:     []byte(`{"message": "Subscriber updated successfully"}`),
			},
		}
		clientObj := &client.Client{Requester: fake}

		err := clientObj.UpdateSubscriber(context.Background(), "001010100000022", &client.UpdateSubscriberOptions{
			ProfileName: "enterprise",
			Description: "Loading dock reader",
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if got := decodeBody(t, fake)["description"]; got != "Loading dock reader" {
			t.Fatalf("description = %q, want %q", got, "Loading dock reader")
		}
	})

	t.Run("an unset description is omitted", func(t *testing.T) {
		fake := &fakeRequester{
			response: &client.RequestResponse{
				StatusCode: 200,
				Headers:    http.Header{},
				Result:     []byte(`{"message": "Subscriber updated successfully"}`),
			},
		}
		clientObj := &client.Client{Requester: fake}

		err := clientObj.UpdateSubscriber(context.Background(), "001010100000022", &client.UpdateSubscriberOptions{
			ProfileName: "enterprise",
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if _, ok := decodeBody(t, fake)["description"]; ok {
			t.Fatal("expected no description key in the request body")
		}
	})
}
