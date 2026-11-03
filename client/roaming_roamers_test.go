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

func TestRoamers(t *testing.T) {
	roamerJSON := `{"imsi": "208930000000001", "partnerID": "abc", "partnerName": "partner", "homePLMN": {"mcc": "208", "mnc": "93"},
		"registrations": [{"rat": "5G", "nodeID": "n1", "reachable": true, "registered": true, "connectionState": "idle", "since": "2026-11-03T10:00:00Z"}],
		"sessions": [{"rat": "5G", "nodeID": "n1", "id": 1, "dataNetwork": "internet", "ipv4Address": "10.45.0.2"}],
		"usageToday": {"uplinkBytes": 1, "downlinkBytes": 2, "totalBytes": 3}}`
	want := client.Roamer{
		IMSI: "208930000000001", PartnerID: "abc", PartnerName: "partner", HomePLMN: client.RoamingPLMN{Mcc: "208", Mnc: "93"},
		Registrations: []client.RoamerRegistration{{RAT: "5G", NodeID: "n1", Reachable: true, Registered: true, ConnectionState: "idle", Since: "2026-11-03T10:00:00Z"}},
		Sessions:      []client.RoamerSession{{RAT: "5G", NodeID: "n1", ID: 1, DataNetwork: "internet", IPv4Address: "10.45.0.2"}},
		UsageToday:    &client.RoamerUsage{UplinkBytes: 1, DownlinkBytes: 2, TotalBytes: 3},
	}

	fake := &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"items": [` + roamerJSON + `], "unreachableNodes": [{"nodeID": "n2", "error": "unreachable"}]}`)}}

	list, err := (&client.Client{Requester: fake}).ListRoamers(context.Background())
	if err != nil || len(list.Items) != 1 || !reflect.DeepEqual(list.Items[0], want) || len(list.UnreachableNodes) != 1 || fake.lastOpts.Path != "api/v1/roaming/roamers" {
		t.Fatalf("ListRoamers = %+v, %v", list, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(roamerJSON)}}

	roamer, err := (&client.Client{Requester: fake}).GetRoamer(context.Background(), "208930000000001")
	if err != nil || !reflect.DeepEqual(*roamer, want) || fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/roaming/roamers/208930000000001" {
		t.Fatalf("GetRoamer = %+v, %v", roamer, err)
	}

	fake = &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"message": "Roamer deregistered successfully"}`)}}

	if err := (&client.Client{Requester: fake}).DeregisterRoamer(context.Background(), "208930000000001"); err != nil || fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/v1/roaming/roamers/208930000000001" || fake.lastOpts.Body != nil {
		t.Fatalf("DeregisterRoamer: %v (%s %s)", err, fake.lastOpts.Method, fake.lastOpts.Path)
	}
}
