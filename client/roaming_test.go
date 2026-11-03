// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestRoamingWrites(t *testing.T) {
	partner := &client.RoamingPartnerOptions{
		Name: "partner", PLMNs: []client.RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileName: "roamers", DiameterPeerID: "peer",
		SEPP: &client.RoamingPartnerSEPP{FQDN: "sepp1.5gc.mnc093.mcc208.3gppnetwork.org", N32cURI: "https://sepp.example.org", TrustAnchorPEM: "pem"},
	}
	peer := &client.RoamingDiameterPeerOptions{DiameterIdentity: "hss.example.org", Address: "192.0.2.1", Port: 3869}
	cert := &client.ImportSEPPCertificateOptions{CertificateChainPEM: "chain", PrivateKeyPEM: "key"}

	cases := []struct {
		name   string
		call   func(c *client.Client) error
		method string
		path   string
		body   map[string]any
	}{
		{"create partner", func(c *client.Client) error {
			_, err := c.CreateRoamingPartner(context.Background(), partner)
			return err
		}, "POST", "api/v1/roaming/partners", map[string]any{
			"name": "partner", "plmns": []any{map[string]any{"mcc": "208", "mnc": "93"}}, "profileName": "roamers", "diameterPeerID": "peer",
			"sepp": map[string]any{"fqdn": "sepp1.5gc.mnc093.mcc208.3gppnetwork.org", "n32cURI": "https://sepp.example.org", "trustAnchorPEM": "pem"},
		}},
		{
			"update partner", func(c *client.Client) error {
				return c.UpdateRoamingPartner(context.Background(), "abc", &client.RoamingPartnerOptions{Name: "p", PLMNs: []client.RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileName: "roamers"})
			},
			"PUT", "api/v1/roaming/partners/abc",
			map[string]any{"name": "p", "plmns": []any{map[string]any{"mcc": "208", "mnc": "93"}}, "profileName": "roamers"},
		},
		{"delete partner", func(c *client.Client) error { return c.DeleteRoamingPartner(context.Background(), "abc") }, "DELETE", "api/v1/roaming/partners/abc", nil},
		{"create peer", func(c *client.Client) error {
			_, err := c.CreateRoamingDiameterPeer(context.Background(), peer)
			return err
		}, "POST", "api/v1/roaming/diameter-peers", map[string]any{"diameterIdentity": "hss.example.org", "address": "192.0.2.1", "port": float64(3869)}},
		{
			"update peer", func(c *client.Client) error { return c.UpdateRoamingDiameterPeer(context.Background(), "abc", peer) },
			"PUT", "api/v1/roaming/diameter-peers/abc",
			map[string]any{"diameterIdentity": "hss.example.org", "address": "192.0.2.1", "port": float64(3869)},
		},
		{"delete peer", func(c *client.Client) error { return c.DeleteRoamingDiameterPeer(context.Background(), "abc") }, "DELETE", "api/v1/roaming/diameter-peers/abc", nil},
		{"import certificate", func(c *client.Client) error {
			_, err := c.ImportSEPPCertificate(context.Background(), cert)
			return err
		}, "PUT", "api/v1/roaming/sepp/certificate", map[string]any{"certificateChainPEM": "chain", "privateKeyPEM": "key"}},
		{"generate certificate", func(c *client.Client) error {
			_, err := c.GenerateSEPPCertificate(context.Background(), nil)
			return err
		}, "POST", "api/v1/roaming/sepp/certificate/generate", map[string]any{}},
		{"generate certificate with an FQDN", func(c *client.Client) error {
			_, err := c.GenerateSEPPCertificate(context.Background(), &client.GenerateSEPPCertificateOptions{FQDN: "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"})
			return err
		}, "POST", "api/v1/roaming/sepp/certificate/generate", map[string]any{"fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{}`)}}

			if err := tc.call(&client.Client{Requester: fake}); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if fake.lastOpts.Method != tc.method || fake.lastOpts.Path != tc.path {
				t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
			}

			if tc.body == nil {
				if fake.lastOpts.Body != nil {
					t.Fatal("unexpected body")
				}

				return
			}

			var payload map[string]any
			if err := json.NewDecoder(fake.lastOpts.Body).Decode(&payload); err != nil {
				t.Fatalf("decode body: %v", err)
			}

			if !reflect.DeepEqual(payload, tc.body) {
				t.Fatalf("payload = %v, want %v", payload, tc.body)
			}
		})
	}
}

func TestRoamingReads(t *testing.T) {
	partnerJSON := `{"id": "abc", "name": "partner", "plmns": [{"mcc": "208", "mnc": "93"}], "profileName": "roamers",
		"sepp": {"fqdn": "sepp1.5gc.mnc093.mcc208.3gppnetwork.org", "n32cURI": "https://sepp.example.org", "trustAnchorPEM": "pem"}}`
	wantPartner := client.RoamingPartner{
		ID: "abc", Name: "partner", PLMNs: []client.RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileName: "roamers",
		SEPP: &client.RoamingPartnerSEPP{FQDN: "sepp1.5gc.mnc093.mcc208.3gppnetwork.org", N32cURI: "https://sepp.example.org", TrustAnchorPEM: "pem"},
	}

	fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"items": [` + partnerJSON + `]}`)}}

	partners, err := (&client.Client{Requester: fake}).ListRoamingPartners(context.Background())
	if err != nil || len(partners.Items) != 1 || !reflect.DeepEqual(partners.Items[0], wantPartner) || fake.lastOpts.Path != "api/v1/roaming/partners" {
		t.Fatalf("ListRoamingPartners = %+v, %v", partners, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(partnerJSON)}}

	partner, err := (&client.Client{Requester: fake}).GetRoamingPartner(context.Background(), "abc")
	if err != nil || !reflect.DeepEqual(*partner, wantPartner) || fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/roaming/partners/abc" {
		t.Fatalf("GetRoamingPartner = %+v, %v", partner, err)
	}

	peerJSON := `{"id": "abc", "diameterIdentity": "hss.example.org", "address": "192.0.2.1", "port": 3868}`
	wantPeer := client.RoamingDiameterPeer{ID: "abc", DiameterIdentity: "hss.example.org", Address: "192.0.2.1", Port: 3868}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"items": [` + peerJSON + `]}`)}}

	peers, err := (&client.Client{Requester: fake}).ListRoamingDiameterPeers(context.Background())
	if err != nil || len(peers.Items) != 1 || peers.Items[0] != wantPeer || fake.lastOpts.Path != "api/v1/roaming/diameter-peers" {
		t.Fatalf("ListRoamingDiameterPeers = %+v, %v", peers, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(peerJSON)}}

	peer, err := (&client.Client{Requester: fake}).GetRoamingDiameterPeer(context.Background(), "abc")
	if err != nil || *peer != wantPeer || fake.lastOpts.Path != "api/v1/roaming/diameter-peers/abc" {
		t.Fatalf("GetRoamingDiameterPeer = %+v, %v", peer, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org", "certificateChainPEM": "chain", "trustAnchorPEM": "ca", "fingerprint": "sha256:00", "notAfter": "2029-01-01T00:00:00Z"}`)}}

	cert, err := (&client.Client{Requester: fake}).GetSEPPCertificate(context.Background())
	if err != nil || cert.FQDN != "sepp1.5gc.mnc001.mcc001.3gppnetwork.org" || cert.TrustAnchorPEM != "ca" || cert.Fingerprint != "sha256:00" || fake.lastOpts.Path != "api/v1/roaming/sepp/certificate" {
		t.Fatalf("GetSEPPCertificate = %+v, %v", cert, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`[{"date": "2026-11-03", "mcc": "208", "mnc": "93", "partner_name": "partner", "uplink_bytes": 1, "downlink_bytes": 2, "total_bytes": 3}]`)}}

	usage, err := (&client.Client{Requester: fake}).ListRoamingUsage(context.Background(), &client.ListRoamingUsageParams{Start: "2026-11-01T00:00:00Z", IMSI: "208930000000001"})
	if err != nil || len(usage) != 1 || usage[0] != (client.RoamingUsage{Date: "2026-11-03", Mcc: "208", Mnc: "93", PartnerName: "partner", UplinkBytes: 1, DownlinkBytes: 2, TotalBytes: 3}) {
		t.Fatalf("ListRoamingUsage = %+v, %v", usage, err)
	}

	if fake.lastOpts.Path != "api/v1/roaming/usage" || !reflect.DeepEqual(fake.lastOpts.Query, url.Values{"start": {"2026-11-01T00:00:00Z"}, "imsi": {"208930000000001"}}) {
		t.Fatalf("unexpected usage request %s %v", fake.lastOpts.Path, fake.lastOpts.Query)
	}
}

func TestRoamingRequestError(t *testing.T) {
	fake := &fakeRequester{err: errors.New("boom")}

	if _, err := (&client.Client{Requester: fake}).GetSEPPCertificate(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}
