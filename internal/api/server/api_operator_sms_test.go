// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"slices"
	"testing"
)

type UpdateOperatorSMSParams struct {
	SMSNumber string `json:"smsNumber"`
}

type SMSCNodeParams struct {
	DiameterIdentity string `json:"diameterIdentity,omitempty"`
	Address          string `json:"address"`
	Port             int    `json:"port,omitempty"`
}

type SMSCParams struct {
	Name           string           `json:"name,omitempty"`
	ServiceCentres []string         `json:"serviceCentres"`
	Nodes          []SMSCNodeParams `json:"nodes"`
}

type UpdateOperatorSMSResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type SMSCResponse struct {
	Result SMSCResultItem `json:"result"`
	Error  string         `json:"error,omitempty"`
}

type ListSMSCsResponse struct {
	Result struct {
		Items []SMSCResultItem `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func updateOperatorSMS(url string, client *http.Client, token string, data *UpdateOperatorSMSParams) (int, *UpdateOperatorSMSResponse, error) {
	return apiDo[UpdateOperatorSMSResponse](client, "PUT", url+"/api/v1/operator/sms", token, data)
}

func createSMSC(url string, client *http.Client, token string, data *SMSCParams) (int, *SMSCResponse, error) {
	return apiDo[SMSCResponse](client, "POST", url+"/api/v1/operator/sms/smscs", token, data)
}

func listSMSCs(url string, client *http.Client, token string) (int, *ListSMSCsResponse, error) {
	return apiDo[ListSMSCsResponse](client, "GET", url+"/api/v1/operator/sms/smscs", token, nil)
}

func getSMSC(url string, client *http.Client, token, name string) (int, *SMSCResponse, error) {
	return apiDo[SMSCResponse](client, "GET", url+"/api/v1/operator/sms/smscs/"+name, token, nil)
}

func updateSMSC(url string, client *http.Client, token, name string, data *SMSCParams) (int, *UpdateOperatorSMSResponse, error) {
	return apiDo[UpdateOperatorSMSResponse](client, "PUT", url+"/api/v1/operator/sms/smscs/"+name, token, data)
}

func deleteSMSC(url string, client *http.Client, token, name string) (int, *UpdateOperatorSMSResponse, error) {
	return apiDo[UpdateOperatorSMSResponse](client, "DELETE", url+"/api/v1/operator/sms/smscs/"+name, token, nil)
}

func oneNode(identity, address string, port int, centres ...string) *SMSCParams {
	return &SMSCParams{ServiceCentres: centres, Nodes: []SMSCNodeParams{{DiameterIdentity: identity, Address: address, Port: port}}}
}

func named(name string, p *SMSCParams) *SMSCParams {
	p.Name = name
	return p
}

func TestUpdateOperatorSMS(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	getSMS := func(t *testing.T) GetOperatorSMSResponseResult {
		t.Helper()

		code, resp, err := getOperator(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("get operator: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("get operator: expected 200, got %d (%q)", code, resp.Error)
		}

		return resp.Result.SMS
	}

	t.Run("the SMS number has a default", func(t *testing.T) {
		if got := getSMS(t); got != (GetOperatorSMSResponseResult{SMSNumber: "+15550001111"}) {
			t.Fatalf("sms = %+v, want the default number", got)
		}
	})

	t.Run("the SMS number is cleared", func(t *testing.T) {
		code, resp, err := updateOperatorSMS(env.Server.URL, client, token, &UpdateOperatorSMSParams{})
		if err != nil || code != http.StatusCreated {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		if got := getSMS(t); got != (GetOperatorSMSResponseResult{}) {
			t.Fatalf("sms = %+v", got)
		}
	})

	t.Run("the SMS number is set", func(t *testing.T) {
		code, resp, err := updateOperatorSMS(env.Server.URL, client, token, &UpdateOperatorSMSParams{SMSNumber: "+15550001111"})
		if err != nil || code != http.StatusCreated {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		if got := getSMS(t); got != (GetOperatorSMSResponseResult{SMSNumber: "+15550001111"}) {
			t.Fatalf("sms = %+v", got)
		}
	})

	t.Run("invalid numbers are rejected", func(t *testing.T) {
		for _, number := range []string{"+0555", "15550001111"} {
			code, resp, err := updateOperatorSMS(env.Server.URL, client, token, &UpdateOperatorSMSParams{SMSNumber: number})
			if err != nil {
				t.Fatalf("%s: %s", number, err)
			}

			if code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d (%q)", number, code, resp.Error)
			}
		}

		if got := getSMS(t); got.SMSNumber != "+15550001111" {
			t.Fatalf("rejected updates changed the settings: %+v", got)
		}
	})
}

func TestSMSCs(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	smscs := func(t *testing.T) []SMSCResultItem {
		t.Helper()

		code, resp, err := listSMSCs(env.Server.URL, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("list SMSCs: code=%d err=%v", code, err)
		}

		return resp.Result.Items
	}

	t.Run("there are no SMSCs by default", func(t *testing.T) {
		if got := smscs(t); got == nil || len(got) != 0 {
			t.Fatalf("SMSCs = %+v, want an empty list", got)
		}
	})

	t.Run("a created SMSC is returned in canonical form", func(t *testing.T) {
		code, resp, err := createSMSC(env.Server.URL, client, token, &SMSCParams{
			Name:           "a",
			ServiceCentres: []string{"+15550000001", "+15550000000"},
			Nodes: []SMSCNodeParams{
				{DiameterIdentity: " smsc-a.example.org ", Address: "2001:DB8:0::10", Port: 3869},
				{DiameterIdentity: "smsc-a2.example.org", Address: "2001:db8::11"},
				{DiameterIdentity: "smsc-a3.example.org", Address: " SMSC-0.smsc.ella.svc.cluster.local "},
			},
		})
		if err != nil || code != http.StatusCreated {
			t.Fatalf("create: code=%d err=%v (%q)", code, err, resp.Error)
		}

		wantNodes := []SMSCNodeResultItem{
			{DiameterIdentity: "smsc-a.example.org", Address: "2001:db8::10", Port: 3869},
			{DiameterIdentity: "smsc-a2.example.org", Address: "2001:db8::11", Port: 3868},
			{DiameterIdentity: "smsc-a3.example.org", Address: "smsc-0.smsc.ella.svc.cluster.local", Port: 3868},
		}

		if created := resp.Result; created.Name != "a" || !slices.Equal(created.Nodes, wantNodes) {
			t.Fatalf("created = %+v", created)
		}

		got := smscs(t)
		if len(got) != 1 || got[0].Name != "a" || !slices.Equal(got[0].ServiceCentres, []string{"+15550000000", "+15550000001"}) || !slices.Equal(got[0].Nodes, wantNodes) {
			t.Fatalf("SMSCs = %+v", got)
		}

		code, one, err := getSMSC(env.Server.URL, client, token, "a")
		if err != nil || code != http.StatusOK || one.Result.Name != "a" {
			t.Fatalf("get: code=%d err=%v (%+v)", code, err, one)
		}

		code, one, err = getSMSC(env.Server.URL, client, token, "A")
		if err != nil || code != http.StatusOK || one.Result.Name != "a" {
			t.Fatalf("get with another case: code=%d err=%v (%+v)", code, err, one)
		}

		code, one, err = getSMSC(env.Server.URL, client, token, "missing")
		if err != nil || code != http.StatusNotFound {
			t.Fatalf("get of a missing SMSC: code=%d err=%v (%+v)", code, err, one)
		}
	})

	t.Run("invalid SMSCs are rejected", func(t *testing.T) {
		cases := map[string]*SMSCParams{
			"no name":                  oneNode("smsc-192-0-2-10.example.org", "192.0.2.10", 0, "+15550000002"),
			"dot dot name":             named("..", oneNode("smsc-192-0-2-10.example.org", "192.0.2.10", 0, "+15550000002")),
			"bad hostname":             named("x", oneNode("smsc-x.example.org", "smsc_x.example.org", 0, "+15550000002")),
			"numeric hostname":         named("x", oneNode("smsc-x.example.org", "192.0.2", 0, "+15550000002")),
			"unspecified":              named("x", oneNode("smsc-0-0-0-0.example.org", "0.0.0.0", 0, "+15550000002")),
			"bad port":                 named("x", oneNode("smsc-192-0-2-10.example.org", "192.0.2.10", 70000, "+15550000002")),
			"no service centre":        named("x", oneNode("smsc-192-0-2-10.example.org", "192.0.2.10", 0)),
			"service centre no plus":   named("x", oneNode("smsc-192-0-2-10.example.org", "192.0.2.10", 0, "15550000002")),
			"bad identity":             named("x", oneNode("smsc", "192.0.2.10", 0, "+15550000002")),
			"duplicate service centre": named("x", oneNode("smsc-192-0-2-10.example.org", "192.0.2.10", 0, "+15550000002", "+15550000002")),
			"no nodes":                 {Name: "x", ServiceCentres: []string{"+15550000002"}},
			"too many nodes":           {Name: "x", ServiceCentres: []string{"+15550000002"}, Nodes: slices.Repeat([]SMSCNodeParams{{Address: "192.0.2.10"}}, 9)},
		}

		for name, params := range cases {
			code, resp, err := createSMSC(env.Server.URL, client, token, params)
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}

			if code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d (%q)", name, code, resp.Error)
			}
		}
	})

	t.Run("conflicting SMSCs are rejected with the reason", func(t *testing.T) {
		cases := map[string]struct {
			params *SMSCParams
			reason string
		}{
			"served service centre": {named("b", oneNode("smsc-192-0-2-11.example.org", "192.0.2.11", 0, "+15550000000")), "another SMSC serves the service centre number +15550000000"},
			"shared identity":       {named("b", oneNode("SMSC-A.example.org", "192.0.2.11", 0, "+15550000002")), "another SMSC node has the Diameter identity SMSC-A.example.org"},
			"shared endpoint":       {named("b", oneNode("smsc-b.example.org", "2001:db8::10", 3869, "+15550000002")), "another SMSC node has the address [2001:db8::10]:3869"},
			"same name":             {named("A", oneNode("smsc-b.example.org", "192.0.2.11", 0, "+15550000002")), "an SMSC with this name already exists"},
		}

		for name, tc := range cases {
			code, resp, err := createSMSC(env.Server.URL, client, token, tc.params)
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}

			if code != http.StatusConflict || resp.Error != tc.reason {
				t.Fatalf("%s: got %d %q, want 409 %q", name, code, resp.Error, tc.reason)
			}
		}

		if got := smscs(t); len(got) != 1 {
			t.Fatalf("rejected SMSCs were stored: %+v", got)
		}
	})

	t.Run("an SMSC is updated", func(t *testing.T) {
		code, resp, err := createSMSC(env.Server.URL, client, token, named("b", oneNode("smsc-b.example.org", "2001:db8::10", 3870, "+15550000002")))
		if err != nil || code != http.StatusCreated {
			t.Fatalf("create: code=%d err=%v (%q)", code, err, resp.Error)
		}

		update := &SMSCParams{
			ServiceCentres: []string{"+15550000003"},
			Nodes: []SMSCNodeParams{
				{DiameterIdentity: "smsc-192-0-2-12.example.org", Address: "192.0.2.12"},
				{DiameterIdentity: "smsc-192-0-2-13.example.org", Address: "192.0.2.13", Port: 3869},
			},
		}

		code, upd, err := updateSMSC(env.Server.URL, client, token, "b", update)
		if err != nil || code != http.StatusOK {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, upd.Error)
		}

		got := smscs(t)[1]
		wantNodes := []SMSCNodeResultItem{
			{DiameterIdentity: "smsc-192-0-2-12.example.org", Address: "192.0.2.12", Port: 3868},
			{DiameterIdentity: "smsc-192-0-2-13.example.org", Address: "192.0.2.13", Port: 3869},
		}

		if got.Name != "b" || !slices.Equal(got.Nodes, wantNodes) || !slices.Equal(got.ServiceCentres, []string{"+15550000003"}) {
			t.Fatalf("SMSC = %+v", got)
		}

		code, upd, err = updateSMSC(env.Server.URL, client, token, "b", oneNode("smsc-192-0-2-12.example.org", "192.0.2.12", 0, "+15550000000"))
		if err != nil || code != http.StatusConflict {
			t.Fatalf("update to a served service centre: code=%d err=%v (%q)", code, err, upd.Error)
		}

		code, upd, err = updateSMSC(env.Server.URL, client, token, "missing", oneNode("smsc-192-0-2-14.example.org", "192.0.2.14", 0, "+15550000004"))
		if err != nil || code != http.StatusNotFound {
			t.Fatalf("update of a missing SMSC: code=%d err=%v (%q)", code, err, upd.Error)
		}
	})

	t.Run("the last SMSC can be deleted", func(t *testing.T) {
		for _, s := range smscs(t) {
			code, resp, err := deleteSMSC(env.Server.URL, client, token, s.Name)
			if err != nil || code != http.StatusOK {
				t.Fatalf("delete: code=%d err=%v (%q)", code, err, resp.Error)
			}
		}

		if got := smscs(t); len(got) != 0 {
			t.Fatalf("SMSCs = %+v, want none", got)
		}

		code, resp, err := deleteSMSC(env.Server.URL, client, token, "missing")
		if err != nil || code != http.StatusNotFound {
			t.Fatalf("delete of a missing SMSC: code=%d err=%v (%q)", code, err, resp.Error)
		}
	})

	t.Run("nodes without an identity take the one they give", func(t *testing.T) {
		code, resp, err := createSMSC(env.Server.URL, client, token, named("c", oneNode("", "192.0.2.20", 0, "+15550000005")))
		if err != nil || code != http.StatusCreated || resp.Result.Nodes[0].DiameterIdentity != "" {
			t.Fatalf("create: code=%d err=%v (%+v)", code, err, resp)
		}

		code, resp, err = createSMSC(env.Server.URL, client, token, named("d", oneNode("", "192.0.2.20", 3869, "+15550000006")))
		if err != nil || code != http.StatusConflict || resp.Error != "another SMSC node without a Diameter identity has the address 192.0.2.20" {
			t.Fatalf("second node without an identity on the same address: code=%d err=%v (%q)", code, err, resp.Error)
		}
	})
}
