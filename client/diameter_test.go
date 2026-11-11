// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetDiameterStatus(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result: []byte(`{
				"realm": "epc.mnc001.mcc001.3gppnetwork.org",
				"servedRealms": ["epc.mnc001.mcc001.3gppnetwork.org", "ims.mnc001.mcc001.3gppnetwork.org"],
				"nodes": [{"nodeId": "1", "originHost": "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org", "local": true}],
				"listener": {"enabled": false, "addresses": []},
				"peers": [{"id": "smsc-a", "role": "smsc", "owner": {"kind": "smsc", "name": "main"}, "originHost": "smsc.example.org", "realm": "example.org", "address": "192.0.2.10", "port": 3868, "direction": "outbound", "state": "open", "since": "2026-09-29T16:00:00Z"}]
			}`),
		},
	}

	status, err := (&client.Client{Requester: fake}).GetDiameterStatus(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/integrations/diameter" {
		t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	want := client.DiameterStatus{
		Realm:        "epc.mnc001.mcc001.3gppnetwork.org",
		ServedRealms: []string{"epc.mnc001.mcc001.3gppnetwork.org", "ims.mnc001.mcc001.3gppnetwork.org"},
		Nodes:        []client.DiameterNodeIdentity{{NodeID: "1", OriginHost: "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org", Local: true}},
		Listener:     client.DiameterListener{Addresses: []string{}},
		Peers: []client.DiameterPeer{{
			ID: "smsc-a", Role: "smsc", Owner: &client.DiameterPeerOwner{Kind: "smsc", Name: "main"}, OriginHost: "smsc.example.org", Realm: "example.org",
			Address: "192.0.2.10", Port: 3868, Direction: "outbound", State: "open", Since: "2026-09-29T16:00:00Z",
		}},
	}

	if !reflect.DeepEqual(*status, want) {
		t.Fatalf("status = %+v, want %+v", *status, want)
	}

	if host := status.LocalOriginHost(); host != "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org" {
		t.Fatalf("local Origin-Host = %q", host)
	}
}
