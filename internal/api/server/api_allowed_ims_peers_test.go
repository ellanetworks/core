// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"testing"
)

const allowedIMSPeersPath = "/api/v1/integrations/ims/allowed-peers"

type AllowedIMSPeerParams struct {
	Name       string   `json:"name,omitempty"`
	OriginHost string   `json:"originHost"`
	Realm      string   `json:"realm"`
	Addresses  []string `json:"addresses,omitempty"`
}

type AllowedIMSPeerItem struct {
	Name        string   `json:"name"`
	OriginHost  string   `json:"originHost"`
	Realm       string   `json:"realm"`
	Addresses   []string `json:"addresses"`
	Connections []any    `json:"connections"`
}

type AllowedIMSPeerResponse struct {
	Result AllowedIMSPeerItem `json:"result"`
	Error  string             `json:"error,omitempty"`
}

type ListAllowedIMSPeersResponse struct {
	Result struct {
		Items []AllowedIMSPeerItem `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func TestAllowedIMSPeers(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL

	list := func(t *testing.T) []AllowedIMSPeerItem {
		t.Helper()

		status, resp, err := apiDo[ListAllowedIMSPeersResponse](client, "GET", url+allowedIMSPeersPath, token, nil)
		if err != nil || status != http.StatusOK {
			t.Fatalf("list: status %d, err %v", status, err)
		}

		return resp.Result.Items
	}

	restricted := func(t *testing.T) bool {
		t.Helper()

		_, resp, err := getIMSSettings(url, client, token)
		if err != nil {
			t.Fatal(err)
		}

		return resp.Result.RestrictPeers
	}

	if got := list(t); len(got) != 0 || restricted(t) {
		t.Fatalf("peers = %+v, restricted %t; want none and unrestricted", got, restricted(t))
	}

	status, resp, err := apiDo[AllowedIMSPeerResponse](client, "POST", url+allowedIMSPeersPath, token, AllowedIMSPeerParams{Name: "scscf", OriginHost: " SCSCF.ims.example.org ", Addresses: []string{"192.0.2.10"}})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create: status %d, err %v (%q)", status, err, resp.Error)
	}

	if resp.Result.OriginHost != "scscf.ims.example.org" || resp.Result.Connections == nil || len(resp.Result.Addresses) != 1 || resp.Result.Addresses[0] != "192.0.2.10/32" {
		t.Fatalf("created = %+v", resp.Result)
	}

	if restricted(t) {
		t.Fatal("adding an allowed IMS peer restricted the peers")
	}

	for name, body := range map[string]AllowedIMSPeerParams{
		"no name":            {OriginHost: "a.example.org", Addresses: []string{"192.0.2.1"}},
		"no host or realm":   {Name: "empty", Addresses: []string{"192.0.2.1"}},
		"bad host":           {Name: "bad", OriginHost: "not a host", Addresses: []string{"192.0.2.1"}},
		"single-label realm": {Name: "bad", Realm: "example", Addresses: []string{"192.0.2.1"}},
		"IP Origin-Host":     {Name: "ip", OriginHost: "192.0.2.1", Addresses: []string{"192.0.2.1"}},
		"name is a dot":      {Name: ".", Realm: "ims.example.org", Addresses: []string{"192.0.2.1"}},
		"no address":         {Name: "addr", Realm: "ims.example.org"},
		"host name address":  {Name: "addr", Realm: "ims.example.org", Addresses: []string{"scscf.example.org"}},
		"bad prefix length":  {Name: "addr", Realm: "ims.example.org", Addresses: []string{"192.0.2.0/33"}},
		"only addresses":     {Name: "addr", Addresses: []string{"192.0.2.1"}},
	} {
		if status, resp, err := apiDo[AllowedIMSPeerResponse](client, "POST", url+allowedIMSPeersPath, token, body); err != nil || status != http.StatusBadRequest {
			t.Fatalf("%s: status %d, err %v (%q); want 400", name, status, err, resp.Error)
		}
	}

	for name, body := range map[string]AllowedIMSPeerParams{
		"same name": {Name: "SCSCF", Realm: "ims.example.org", Addresses: []string{"192.0.2.1"}},
		"same rule": {Name: "other", OriginHost: "scscf.ims.example.org", Addresses: []string{"192.0.2.10/32"}},
	} {
		if status, resp, err := apiDo[AllowedIMSPeerResponse](client, "POST", url+allowedIMSPeersPath, token, body); err != nil || status != http.StatusConflict {
			t.Fatalf("%s: status %d, err %v (%q); want 409", name, status, err, resp.Error)
		}
	}

	if status, resp, err := apiDo[AllowedIMSPeerResponse](client, "POST", url+allowedIMSPeersPath, token, AllowedIMSPeerParams{Name: "scscf-b", OriginHost: "scscf.ims.example.org", Addresses: []string{"192.0.2.0/24"}}); err != nil || status != http.StatusCreated {
		t.Fatalf("same Origin-Host from other addresses: status %d, err %v (%q)", status, err, resp.Error)
	}

	if status, resp, err := apiDo[AllowedIMSPeerResponse](client, "PUT", url+allowedIMSPeersPath+"/scscf", token, AllowedIMSPeerParams{Realm: "ims.example.org", Addresses: []string{"2001:db8::7", "198.51.100.9/24"}}); err != nil || status != http.StatusOK {
		t.Fatalf("update: status %d, err %v (%q)", status, err, resp.Error)
	}

	status, resp, err = apiDo[AllowedIMSPeerResponse](client, "GET", url+allowedIMSPeersPath+"/scscf", token, nil)
	if err != nil || status != http.StatusOK || resp.Result.OriginHost != "" || resp.Result.Realm != "ims.example.org" ||
		len(resp.Result.Addresses) != 2 || resp.Result.Addresses[0] != "198.51.100.0/24" || resp.Result.Addresses[1] != "2001:db8::7/128" {
		t.Fatalf("get: status %d, err %v, peer %+v", status, err, resp.Result)
	}

	if status, _, err := apiDo[AllowedIMSPeerResponse](client, "DELETE", url+allowedIMSPeersPath+"/scscf-b", token, nil); err != nil || status != http.StatusOK {
		t.Fatalf("delete scscf-b: status %d, err %v", status, err)
	}

	if status, _, err := apiDo[AllowedIMSPeerResponse](client, "GET", url+allowedIMSPeersPath+"/missing", token, nil); err != nil || status != http.StatusNotFound {
		t.Fatalf("get missing: status %d, err %v", status, err)
	}

	readOnly := newTestClient(env.Server)

	readOnlyToken, err := createUserAndLogin(url, token, "readonly@ellanetworks.com", RoleReadOnly, readOnly)
	if err != nil {
		t.Fatalf("create read-only user: %s", err)
	}

	if status, _, err := apiDo[ListAllowedIMSPeersResponse](readOnly, "GET", url+allowedIMSPeersPath, readOnlyToken, nil); err != nil || status != http.StatusOK {
		t.Fatalf("read-only list: status %d, err %v", status, err)
	}

	if status, _, err := apiDo[AllowedIMSPeerResponse](readOnly, "POST", url+allowedIMSPeersPath, readOnlyToken, AllowedIMSPeerParams{Name: "ro", Realm: "ro.example.org", Addresses: []string{"192.0.2.1"}}); err != nil || status != http.StatusForbidden {
		t.Fatalf("read-only create: status %d, err %v", status, err)
	}

	if code, resp, err := updateIMSSettings(url, client, token, &IMSSettingsParams{PCSCFAddresses: []string{}, AdvertiseVoice: true, RestrictPeers: true}); err != nil || code != http.StatusOK {
		t.Fatalf("restrict peers: code %d, err %v (%q)", code, err, resp.Error)
	}

	if status, _, err := apiDo[AllowedIMSPeerResponse](client, "DELETE", url+allowedIMSPeersPath+"/scscf", token, nil); err != nil || status != http.StatusOK {
		t.Fatalf("delete: status %d, err %v", status, err)
	}

	if got := list(t); len(got) != 0 || !restricted(t) {
		t.Fatalf("after delete: peers = %+v, restricted %t; want none and still restricted", got, restricted(t))
	}
}
