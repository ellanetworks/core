// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

func TestRoamingWrites(t *testing.T) {
	partner := &client.RoamingPartnerOptions{
		Name: "partner", PLMNs: []client.RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileName: "roamers", DiameterPeerID: "peer",
		SEPP: &client.RoamingPartnerSEPP{FQDN: "sepp1.5gc.mnc093.mcc208.3gppnetwork.org", N32cURI: "https://sepp.example.org", TrustAnchorPEM: "pem"},
	}
	peer := &client.RoamingDiameterPeerOptions{OriginHost: "hss.example.org", Address: "192.0.2.1", Port: 3869}
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
		}, "POST", "api/v1/integrations/roaming/partners", map[string]any{
			"name": "partner", "plmns": []any{map[string]any{"mcc": "208", "mnc": "93"}}, "profileName": "roamers", "diameterPeerID": "peer",
			"sepp": map[string]any{"fqdn": "sepp1.5gc.mnc093.mcc208.3gppnetwork.org", "n32cURI": "https://sepp.example.org", "trustAnchorPEM": "pem"},
		}},
		{
			"update partner", func(c *client.Client) error {
				return c.UpdateRoamingPartner(context.Background(), "abc", &client.RoamingPartnerOptions{Name: "p", PLMNs: []client.RoamingPLMN{{Mcc: "208", Mnc: "93"}}, ProfileName: "roamers"})
			},
			"PUT", "api/v1/integrations/roaming/partners/abc",
			map[string]any{"name": "p", "plmns": []any{map[string]any{"mcc": "208", "mnc": "93"}}, "profileName": "roamers"},
		},
		{"delete partner", func(c *client.Client) error { return c.DeleteRoamingPartner(context.Background(), "abc") }, "DELETE", "api/v1/integrations/roaming/partners/abc", nil},
		{"create peer", func(c *client.Client) error {
			_, err := c.CreateRoamingDiameterPeer(context.Background(), peer)
			return err
		}, "POST", "api/v1/integrations/roaming/diameter-peers", map[string]any{"originHost": "hss.example.org", "address": "192.0.2.1", "port": float64(3869)}},
		{
			"update peer", func(c *client.Client) error { return c.UpdateRoamingDiameterPeer(context.Background(), "abc", peer) },
			"PUT", "api/v1/integrations/roaming/diameter-peers/abc",
			map[string]any{"originHost": "hss.example.org", "address": "192.0.2.1", "port": float64(3869)},
		},
		{"delete peer", func(c *client.Client) error { return c.DeleteRoamingDiameterPeer(context.Background(), "abc") }, "DELETE", "api/v1/integrations/roaming/diameter-peers/abc", nil},
		{"import certificate", func(c *client.Client) error {
			_, err := c.ImportSEPPCertificate(context.Background(), 2, cert)
			return err
		}, "PUT", "api/v1/integrations/roaming/sepp/nodes/2/certificate", map[string]any{"certificateChainPEM": "chain", "privateKeyPEM": "key"}},
		{"delete certificate", func(c *client.Client) error {
			return c.DeleteSEPPCertificate(context.Background(), 3)
		}, "DELETE", "api/v1/integrations/roaming/sepp/nodes/3/certificate", nil},
		{"regenerate CA", func(c *client.Client) error {
			_, err := c.RegenerateSEPPCA(context.Background())
			return err
		}, "POST", "api/v1/integrations/roaming/sepp/ca/regenerate", nil},
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

			withdraws := strings.HasPrefix(tc.path, "api/v1/integrations/roaming/partners/") && (tc.method == "PUT" || tc.method == "DELETE")
			if withdraws != (fake.lastOpts.Timeout > time.Minute) {
				t.Fatalf("timeout = %s for %s %s", fake.lastOpts.Timeout, tc.method, tc.path)
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
	if err != nil || len(partners.Items) != 1 || !reflect.DeepEqual(partners.Items[0], wantPartner) || fake.lastOpts.Path != "api/v1/integrations/roaming/partners" {
		t.Fatalf("ListRoamingPartners = %+v, %v", partners, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(partnerJSON)}}

	partner, err := (&client.Client{Requester: fake}).GetRoamingPartner(context.Background(), "abc")
	if err != nil || !reflect.DeepEqual(*partner, wantPartner) || fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/integrations/roaming/partners/abc" {
		t.Fatalf("GetRoamingPartner = %+v, %v", partner, err)
	}

	peerJSON := `{"id": "abc", "originHost": "hss.example.org", "address": "192.0.2.1", "port": 3868}`
	wantPeer := client.RoamingDiameterPeer{ID: "abc", OriginHost: "hss.example.org", Address: "192.0.2.1", Port: 3868}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"items": [` + peerJSON + `]}`)}}

	peers, err := (&client.Client{Requester: fake}).ListRoamingDiameterPeers(context.Background())
	if err != nil || len(peers.Items) != 1 || peers.Items[0] != wantPeer || fake.lastOpts.Path != "api/v1/integrations/roaming/diameter-peers" {
		t.Fatalf("ListRoamingDiameterPeers = %+v, %v", peers, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(peerJSON)}}

	peer, err := (&client.Client{Requester: fake}).GetRoamingDiameterPeer(context.Background(), "abc")
	if err != nil || *peer != wantPeer || fake.lastOpts.Path != "api/v1/integrations/roaming/diameter-peers/abc" {
		t.Fatalf("GetRoamingDiameterPeer = %+v, %v", peer, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"seppID": 1, "source": "imported", "fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org", "fingerprint": "sha256:00", "trustAnchorFingerprint": "sha256:ca", "notAfter": "2029-01-01T00:00:00Z", "warnings": ["w"]}`)}}

	cert, err := (&client.Client{Requester: fake}).ImportSEPPCertificate(context.Background(), 1, &client.ImportSEPPCertificateOptions{CertificateChainPEM: "chain", PrivateKeyPEM: "key"})
	if err != nil || cert.SEPPID != 1 || cert.Source != "imported" || cert.FQDN != "sepp1.5gc.mnc001.mcc001.3gppnetwork.org" || cert.TrustAnchorFingerprint != "sha256:ca" || cert.Fingerprint != "sha256:00" || len(cert.Warnings) != 1 {
		t.Fatalf("ImportSEPPCertificate = %+v, %v", cert, err)
	}
}

func TestRoamingSEPPRuntime(t *testing.T) {
	fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 202, Headers: http.Header{}, Result: []byte(`{"n32": {"state": "establishing", "lastError": "e"}}`)}}

	st, err := (&client.Client{Requester: fake}).RehandshakeRoamingPartner(context.Background(), "abc")
	if err != nil || st.N32.State != "establishing" || st.N32.LastError != "e" || fake.lastOpts.Method != "POST" || fake.lastOpts.Path != "api/v1/integrations/roaming/partners/abc/handshake" || fake.lastOpts.Body != nil {
		t.Fatalf("RehandshakeRoamingPartner = %+v, %v", st, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{
		"trustAnchors": [{"source": "generated", "subject": "CN=ca", "fingerprint": "sha256:ca", "notAfter": "c", "pem": "pem", "seppIDs": [1]}],
		"listener": {"seppID": 1, "configured": true, "address": "192.0.2.1", "port": 443, "listening": true},
		"certificates": [{"seppID": 1, "fqdn": "sepp1.5gc.mnc001.mcc001.3gppnetwork.org", "source": "generated", "subject": "CN=x", "sans": ["sepp1.5gc.mnc001.mcc001.3gppnetwork.org"],
			"issuer": "CN=ca", "notBefore": "a", "notAfter": "b", "fingerprint": "sha256:00", "trustAnchorFingerprint": "sha256:ca"},
			{"seppID": 2, "fqdn": "sepp2.5gc.mnc001.mcc001.3gppnetwork.org", "source": "imported"}]}`)}}

	status, err := (&client.Client{Requester: fake}).GetSEPPStatus(context.Background())
	if err != nil || len(status.TrustAnchors) != 1 || status.TrustAnchors[0].SEPPIDs[0] != 1 || len(status.Certificates) != 2 || status.Certificates[1].SEPPID != 2 || fake.lastOpts.Path != "api/v1/integrations/roaming/sepp" {
		t.Fatalf("GetSEPPStatus = %+v, %v", status, err)
	}

	if l, c := status.Listener, status.Certificates[0]; l.SEPPID != 1 || !l.Configured || !l.Listening || l.Port != 443 || c.FQDN != "sepp1.5gc.mnc001.mcc001.3gppnetwork.org" || c.TrustAnchorFingerprint != "sha256:ca" {
		t.Fatalf("listener = %+v, certificate = %+v", l, c)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"id": "abc", "name": "p", "plmns": [], "profileName": "x",
		"n32": {"state": "established", "peer": "sepp2.5gc.mnc001.mcc001.3gppnetwork.org"}}`)}}

	partner, err := (&client.Client{Requester: fake}).GetRoamingPartner(context.Background(), "abc")
	want := client.RoamingPartnerN32{State: "established", Peer: "sepp2.5gc.mnc001.mcc001.3gppnetwork.org"}

	if err != nil || partner.N32 == nil || !reflect.DeepEqual(*partner.N32, want) {
		t.Fatalf("partner n32 = %+v, %v", partner, err)
	}
}

func TestRoamingRequestError(t *testing.T) {
	fake := &fakeRequester{err: errors.New("boom")}

	if _, err := (&client.Client{Requester: fake}).GetSEPPStatus(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}
