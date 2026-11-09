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

func TestUpdateIMSSettings(t *testing.T) {
	fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"message": "IMS settings updated successfully"}`)}}

	err := (&client.Client{Requester: fake}).UpdateIMSSettings(context.Background(), &client.UpdateIMSSettingsOptions{
		PCSCFAddresses: []string{"192.0.2.20", "2001:db8::20"},
		HomeDomain:     "ims.example.com",
		AdvertiseVoice: true,
		RestrictPeers:  true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "PUT" || fake.lastOpts.Path != "api/v1/integrations/ims" {
		t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	var payload map[string]any
	if err := json.NewDecoder(fake.lastOpts.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	want := map[string]any{"pcscfAddresses": []any{"192.0.2.20", "2001:db8::20"}, "homeDomain": "ims.example.com", "advertiseVoice": true, "restrictPeers": true}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestUpdateIMSSettingsSendsEveryField(t *testing.T) {
	fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{}`)}}

	if err := (&client.Client{Requester: fake}).UpdateIMSSettings(context.Background(), &client.UpdateIMSSettingsOptions{}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var payload map[string]any
	if err := json.NewDecoder(fake.lastOpts.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if addresses, ok := payload["pcscfAddresses"].([]any); !ok || len(addresses) != 0 || payload["homeDomain"] != "" || payload["advertiseVoice"] != false || payload["restrictPeers"] != false {
		t.Fatalf("payload = %v, want an empty list, an empty home domain, advertiseVoice false and restrictPeers false", payload)
	}
}

func TestGetIMSSettings(t *testing.T) {
	fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"pcscfAddresses": ["192.0.2.20"], "voiceStatus": "enabled", "homeDomain": "ims.example.com", "derivedHomeDomain": "ims.mnc001.mcc001.3gppnetwork.org", "advertiseVoice": true, "restrictPeers": true}`)}}

	settings, err := (&client.Client{Requester: fake}).GetIMSSettings(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := client.IMSSettings{PCSCFAddresses: []string{"192.0.2.20"}, VoiceStatus: "enabled", HomeDomain: "ims.example.com", DerivedHomeDomain: "ims.mnc001.mcc001.3gppnetwork.org", AdvertiseVoice: true, RestrictPeers: true}
	if fake.lastOpts.Path != "api/v1/integrations/ims" || !reflect.DeepEqual(*settings, want) {
		t.Fatalf("settings = %+v via %s", settings, fake.lastOpts.Path)
	}
}

func TestIMSPeerRequests(t *testing.T) {
	cases := []struct {
		name   string
		call   func(c *client.Client) error
		method string
		path   string
	}{
		{"list", func(c *client.Client) error { _, err := c.ListAllowedIMSPeers(context.Background()); return err }, "GET", "api/v1/integrations/ims/allowed-peers"},
		{"get", func(c *client.Client) error { _, err := c.GetAllowedIMSPeer(context.Background(), "a/b"); return err }, "GET", "api/v1/integrations/ims/allowed-peers/a%2Fb"},
		{"create", func(c *client.Client) error {
			_, err := c.CreateAllowedIMSPeer(context.Background(), &client.CreateAllowedIMSPeerOptions{Name: "scscf", Realm: "ims.example.org"})
			return err
		}, "POST", "api/v1/integrations/ims/allowed-peers"},
		{"update", func(c *client.Client) error {
			return c.UpdateAllowedIMSPeer(context.Background(), "scscf", &client.UpdateAllowedIMSPeerOptions{OriginHost: "scscf.ims.example.org"})
		}, "PUT", "api/v1/integrations/ims/allowed-peers/scscf"},
		{"delete", func(c *client.Client) error { return c.DeleteAllowedIMSPeer(context.Background(), "scscf") }, "DELETE", "api/v1/integrations/ims/allowed-peers/scscf"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"name": "scscf", "connections": [], "items": []}`)}}

			if err := tc.call(&client.Client{Requester: fake}); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if fake.lastOpts.Method != tc.method || fake.lastOpts.Path != tc.path {
				t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
			}
		})
	}
}
