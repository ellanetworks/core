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
				"host": "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org",
				"realm": "epc.mnc001.mcc001.3gppnetwork.org",
				"peers": [{"role": "smsc", "host": "smsc.example.org", "realm": "example.org", "address": "192.0.2.10", "port": 3868, "state": "open", "since": "2026-09-29T16:00:00Z"}]
			}`),
		},
	}

	status, err := (&client.Client{Requester: fake}).GetDiameterStatus(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/networking/diameter" {
		t.Fatalf("unexpected request %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	want := client.DiameterStatus{
		Host:  "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org",
		Realm: "epc.mnc001.mcc001.3gppnetwork.org",
		Peers: []client.DiameterPeer{{
			Role: "smsc", Host: "smsc.example.org", Realm: "example.org",
			Address: "192.0.2.10", Port: 3868, State: "open", Since: "2026-09-29T16:00:00Z",
		}},
	}

	if !reflect.DeepEqual(*status, want) {
		t.Fatalf("status = %+v, want %+v", *status, want)
	}
}
