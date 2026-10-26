// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/client"
)

func imsFake(result string) *fakeRequester {
	return &fakeRequester{response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(result)}}
}

func decodeBody(t *testing.T, body io.Reader) map[string]any {
	t.Helper()

	var payload map[string]any

	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	return payload
}

func TestIMSApplicationServerReads(t *testing.T) {
	as := `{"name": "mcptt", "server_name": "sip:mcptt.ims.example.org", "public_service_identities": ["sip:!.*!@mcptt.ims.example.org"]}`
	want := client.IMSApplicationServer{Name: "mcptt", ServerName: "sip:mcptt.ims.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.ims.example.org"}}

	fake := imsFake(`{"items": [` + as + `]}`)

	list, err := (&client.Client{Requester: fake}).ListIMSApplicationServers(context.Background())
	if err != nil || len(list.Items) != 1 || !reflect.DeepEqual(list.Items[0], want) {
		t.Fatalf("list = %+v, %v", list, err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/operator/ims/application-servers" {
		t.Fatalf("list via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	fake = imsFake(as)

	got, err := (&client.Client{Requester: fake}).GetIMSApplicationServer(context.Background(), "mcptt")
	if err != nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("get = %+v, %v", got, err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/operator/ims/application-servers/mcptt" {
		t.Fatalf("get via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestIMSApplicationServerWrites(t *testing.T) {
	fake := imsFake(`{"name": "as", "server_name": "sip:as.example.org", "public_service_identities": []}`)
	c := &client.Client{Requester: fake}

	created, err := c.CreateIMSApplicationServer(context.Background(), &client.CreateIMSApplicationServerOptions{Name: "as", ServerName: "sip:as.example.org"})
	if err != nil || created.Name != "as" {
		t.Fatalf("create = %+v, %v", created, err)
	}

	if fake.lastOpts.Method != "POST" || fake.lastOpts.Path != "api/v1/operator/ims/application-servers" {
		t.Fatalf("create via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	body := decodeBody(t, fake.lastOpts.Body)
	if body["name"] != "as" || body["server_name"] != "sip:as.example.org" || !reflect.DeepEqual(body["public_service_identities"], []any{}) {
		t.Fatalf("create body = %+v", body)
	}

	if err := c.UpdateIMSApplicationServer(context.Background(), "as", &client.UpdateIMSApplicationServerOptions{ServerName: "sip:as2.example.org", PublicServiceIdentities: []string{"sip:svc@example.org"}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if fake.lastOpts.Method != "PUT" || fake.lastOpts.Path != "api/v1/operator/ims/application-servers/as" {
		t.Fatalf("update via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	body = decodeBody(t, fake.lastOpts.Body)
	if _, ok := body["name"]; ok || body["server_name"] != "sip:as2.example.org" || !reflect.DeepEqual(body["public_service_identities"], []any{"sip:svc@example.org"}) {
		t.Fatalf("update body = %+v", body)
	}

	if err := c.DeleteIMSApplicationServer(context.Background(), "as"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/v1/operator/ims/application-servers/as" || fake.lastOpts.Body != nil {
		t.Fatalf("delete via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestProfileIMSFilterCriteria(t *testing.T) {
	fake := imsFake(`{"initial_filter_criteria": [
		{"priority": 0, "application_server_name": "mcptt", "trigger": {"conditions_match": "all", "groups": [{"conditions": [{"method": "REGISTER"}]}]},
		 "default_handling": "SESSION_CONTINUED", "service_info": "info", "include_register_request": true, "include_register_response": true},
		{"priority": 1, "application_server_name": "conf", "profile_part": "UNREGISTERED", "include_register_request": false, "include_register_response": false}]}`)
	c := &client.Client{Requester: fake}

	register, handling, info, part := "REGISTER", "SESSION_CONTINUED", "info", "UNREGISTERED"
	want := []client.IMSInitialFilterCriterion{
		{
			ApplicationServerName:   "mcptt",
			Trigger:                 &client.IMSTrigger{ConditionsMatch: "all", Groups: []client.IMSConditionGroup{{Conditions: []client.IMSCondition{{Method: &register}}}}},
			DefaultHandling:         &handling,
			ServiceInfo:             &info,
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{Priority: 1, ApplicationServerName: "conf", ProfilePart: &part},
	}

	got, err := c.GetProfileIMSFilterCriteria(context.Background(), "mc")
	if err != nil || !reflect.DeepEqual(got.InitialFilterCriteria, want) {
		t.Fatalf("get = %+v, %v", got, err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/profiles/mc/ims-filter-criteria" {
		t.Fatalf("get via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	if err := c.SetProfileIMSFilterCriteria(context.Background(), "mc", nil); err != nil {
		t.Fatalf("clear: %v", err)
	}

	body := decodeBody(t, fake.lastOpts.Body)
	if fake.lastOpts.Method != "PUT" || fake.lastOpts.Path != "api/v1/profiles/mc/ims-filter-criteria" || !reflect.DeepEqual(body["initial_filter_criteria"], []any{}) {
		t.Fatalf("clear via %s %s with %+v", fake.lastOpts.Method, fake.lastOpts.Path, body)
	}

	if err := c.SetProfileIMSFilterCriteria(context.Background(), "mc", want); err != nil {
		t.Fatalf("set: %v", err)
	}

	var sent client.ProfileIMSFilterCriteria

	if err := json.NewDecoder(fake.lastOpts.Body).Decode(&sent); err != nil || !reflect.DeepEqual(sent.InitialFilterCriteria, want) {
		t.Fatalf("set body = %+v, %v", sent, err)
	}
}

func TestIMSRequestError(t *testing.T) {
	fake := &fakeRequester{err: errors.New("Application server not found")}

	if _, err := (&client.Client{Requester: fake}).GetIMSApplicationServer(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error")
	}
}
