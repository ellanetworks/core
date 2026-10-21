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

func TestIMSServiceReads(t *testing.T) {
	svc := `{"name": "mcptt", "kind": "mcptt", "application_server_name": "mcptt", "initial_filter_criteria": [
		{"priority": 0, "trigger_point": {"condition_type_cnf": false, "spt": [{"group": [0], "method": "REGISTER"}]},
		 "default_handling": "SESSION_CONTINUED", "include_register_request": true, "include_register_response": true}]}`
	register := "REGISTER"
	want := client.IMSService{
		Name: "mcptt", Kind: "mcptt", ApplicationServerName: "mcptt",
		InitialFilterCriteria: []client.IMSInitialFilterCriterion{{
			TriggerPoint:            client.IMSTriggerPoint{SPT: []client.IMSServicePointTrigger{{Group: []int32{0}, Method: &register}}},
			DefaultHandling:         "SESSION_CONTINUED",
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		}},
	}

	fake := imsFake(`{"items": [` + svc + `]}`)

	list, err := (&client.Client{Requester: fake}).ListIMSServices(context.Background())
	if err != nil || len(list.Items) != 1 || !reflect.DeepEqual(list.Items[0], want) {
		t.Fatalf("list = %+v, %v", list, err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/operator/ims/services" {
		t.Fatalf("list via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	fake = imsFake(svc)

	got, err := (&client.Client{Requester: fake}).GetIMSService(context.Background(), "mcptt")
	if err != nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("get = %+v, %v", got, err)
	}

	if fake.lastOpts.Path != "api/v1/operator/ims/services/mcptt" {
		t.Fatalf("get via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestIMSServiceWrites(t *testing.T) {
	fake := imsFake(`{"name": "mcptt", "kind": "mcptt", "application_server_name": "mcptt", "initial_filter_criteria": []}`)
	c := &client.Client{Requester: fake}

	if _, err := c.CreateIMSService(context.Background(), &client.CreateIMSServiceOptions{Name: "mcptt", Kind: "mcptt", ApplicationServerName: "mcptt"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if fake.lastOpts.Method != "POST" || fake.lastOpts.Path != "api/v1/operator/ims/services" {
		t.Fatalf("create via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	body := decodeBody(t, fake.lastOpts.Body)
	if _, ok := body["initial_filter_criteria"]; ok || body["kind"] != "mcptt" || body["application_server_name"] != "mcptt" {
		t.Fatalf("create body = %+v", body)
	}

	uri := "^sip:conf@.*$"
	custom := &client.UpdateIMSServiceOptions{
		Kind: "custom", ApplicationServerName: "conf",
		InitialFilterCriteria: []client.IMSInitialFilterCriterion{{
			TriggerPoint:    client.IMSTriggerPoint{SPT: []client.IMSServicePointTrigger{{Group: []int32{0}, RequestURI: &uri}}},
			DefaultHandling: "SESSION_TERMINATED",
		}},
	}

	if err := c.UpdateIMSService(context.Background(), "conf", custom); err != nil {
		t.Fatalf("update: %v", err)
	}

	if fake.lastOpts.Method != "PUT" || fake.lastOpts.Path != "api/v1/operator/ims/services/conf" {
		t.Fatalf("update via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	var sent client.UpdateIMSServiceOptions

	if err := json.NewDecoder(fake.lastOpts.Body).Decode(&sent); err != nil || !reflect.DeepEqual(&sent, custom) {
		t.Fatalf("update body = %+v, %v", sent, err)
	}

	if err := c.DeleteIMSService(context.Background(), "conf"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/v1/operator/ims/services/conf" {
		t.Fatalf("delete via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestProfileIMSServices(t *testing.T) {
	fake := imsFake(`{"services": ["mcptt", "conf"]}`)
	c := &client.Client{Requester: fake}

	got, err := c.GetProfileIMSServices(context.Background(), "mc")
	if err != nil || !reflect.DeepEqual(got.Services, []string{"mcptt", "conf"}) {
		t.Fatalf("get = %+v, %v", got, err)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/profiles/mc/ims-services" {
		t.Fatalf("get via %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	if err := c.SetProfileIMSServices(context.Background(), "mc", nil); err != nil {
		t.Fatalf("set: %v", err)
	}

	body := decodeBody(t, fake.lastOpts.Body)
	if fake.lastOpts.Method != "PUT" || fake.lastOpts.Path != "api/v1/profiles/mc/ims-services" || !reflect.DeepEqual(body["services"], []any{}) {
		t.Fatalf("set via %s %s with %+v", fake.lastOpts.Method, fake.lastOpts.Path, body)
	}
}

func TestIMSRequestError(t *testing.T) {
	fake := &fakeRequester{err: errors.New("Application server not found")}

	if _, err := (&client.Client{Requester: fake}).GetIMSApplicationServer(context.Background(), "missing"); err == nil {
		t.Fatal("expected an error")
	}
}
