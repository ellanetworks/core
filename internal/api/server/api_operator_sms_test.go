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

type SMSCPeerParams struct {
	DiameterIdentity string   `json:"diameterIdentity,omitempty"`
	Address          string   `json:"address"`
	Port             int      `json:"port,omitempty"`
	ServiceCentres   []string `json:"serviceCentres"`
}

type UpdateOperatorSMSResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type SMSCPeerResponse struct {
	Result SMSCPeerResultItem `json:"result"`
	Error  string             `json:"error,omitempty"`
}

type ListSMSCPeersResponse struct {
	Result struct {
		Items []SMSCPeerResultItem `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func updateOperatorSMS(url string, client *http.Client, token string, data *UpdateOperatorSMSParams) (int, *UpdateOperatorSMSResponse, error) {
	return apiDo[UpdateOperatorSMSResponse](client, "PUT", url+"/api/v1/operator/sms", token, data)
}

func createSMSCPeer(url string, client *http.Client, token string, data *SMSCPeerParams) (int, *SMSCPeerResponse, error) {
	return apiDo[SMSCPeerResponse](client, "POST", url+"/api/v1/operator/sms/smsc-peers", token, data)
}

func listSMSCPeers(url string, client *http.Client, token string) (int, *ListSMSCPeersResponse, error) {
	return apiDo[ListSMSCPeersResponse](client, "GET", url+"/api/v1/operator/sms/smsc-peers", token, nil)
}

func getSMSCPeer(url string, client *http.Client, token, id string) (int, *SMSCPeerResponse, error) {
	return apiDo[SMSCPeerResponse](client, "GET", url+"/api/v1/operator/sms/smsc-peers/"+id, token, nil)
}

func updateSMSCPeer(url string, client *http.Client, token, id string, data *SMSCPeerParams) (int, *UpdateOperatorSMSResponse, error) {
	return apiDo[UpdateOperatorSMSResponse](client, "PUT", url+"/api/v1/operator/sms/smsc-peers/"+id, token, data)
}

func deleteSMSCPeer(url string, client *http.Client, token, id string) (int, *UpdateOperatorSMSResponse, error) {
	return apiDo[UpdateOperatorSMSResponse](client, "DELETE", url+"/api/v1/operator/sms/smsc-peers/"+id, token, nil)
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

func TestSMSCPeers(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	peers := func(t *testing.T) []SMSCPeerResultItem {
		t.Helper()

		code, resp, err := listSMSCPeers(env.Server.URL, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("list peers: code=%d err=%v", code, err)
		}

		return resp.Result.Items
	}

	t.Run("there are no peers by default", func(t *testing.T) {
		if got := peers(t); got == nil || len(got) != 0 {
			t.Fatalf("peers = %+v, want an empty list", got)
		}
	})

	t.Run("a created peer is returned in canonical form", func(t *testing.T) {
		code, resp, err := createSMSCPeer(env.Server.URL, client, token, &SMSCPeerParams{DiameterIdentity: " smsc-a.example.org ", Address: "2001:DB8:0::10", Port: 3869, ServiceCentres: []string{"+15550000001", "+15550000000"}})
		if err != nil || code != http.StatusCreated {
			t.Fatalf("create: code=%d err=%v (%q)", code, err, resp.Error)
		}

		created := resp.Result
		if created.ID == "" || created.DiameterIdentity != "smsc-a.example.org" || created.Address != "2001:db8::10" || created.Port != 3869 {
			t.Fatalf("created = %+v", created)
		}

		got := peers(t)
		if len(got) != 1 || got[0].ID != created.ID || !slices.Equal(got[0].ServiceCentres, []string{"+15550000000", "+15550000001"}) {
			t.Fatalf("peers = %+v", got)
		}

		code, one, err := getSMSCPeer(env.Server.URL, client, token, created.ID)
		if err != nil || code != http.StatusOK || one.Result.ID != created.ID {
			t.Fatalf("get: code=%d err=%v (%+v)", code, err, one)
		}

		code, one, err = getSMSCPeer(env.Server.URL, client, token, "missing")
		if err != nil || code != http.StatusNotFound {
			t.Fatalf("get of a missing peer: code=%d err=%v (%+v)", code, err, one)
		}
	})

	t.Run("invalid peers are rejected", func(t *testing.T) {
		cases := map[string]SMSCPeerParams{
			"hostname":                 {DiameterIdentity: "smsc-smsc-example-org.example.org", Address: "smsc.example.org", ServiceCentres: []string{"+15550000002"}},
			"unspecified":              {DiameterIdentity: "smsc-0-0-0-0.example.org", Address: "0.0.0.0", ServiceCentres: []string{"+15550000002"}},
			"bad port":                 {DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10", Port: 70000, ServiceCentres: []string{"+15550000002"}},
			"no service centre":        {DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10"},
			"service centre no plus":   {DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10", ServiceCentres: []string{"15550000002"}},
			"bad identity":             {DiameterIdentity: "smsc", Address: "192.0.2.10", ServiceCentres: []string{"+15550000002"}},
			"duplicate service centre": {DiameterIdentity: "smsc-192-0-2-10.example.org", Address: "192.0.2.10", ServiceCentres: []string{"+15550000002", "+15550000002"}},
		}

		for name, params := range cases {
			code, resp, err := createSMSCPeer(env.Server.URL, client, token, &params)
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}

			if code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d (%q)", name, code, resp.Error)
			}
		}
	})

	t.Run("conflicting peers are rejected with the reason", func(t *testing.T) {
		cases := map[string]struct {
			params SMSCPeerParams
			reason string
		}{
			"served service centre": {SMSCPeerParams{DiameterIdentity: "smsc-192-0-2-11.example.org", Address: "192.0.2.11", ServiceCentres: []string{"+15550000000"}}, "another SMSC peer serves the service centre number +15550000000"},
			"shared identity":       {SMSCPeerParams{DiameterIdentity: "SMSC-A.example.org", Address: "192.0.2.11", ServiceCentres: []string{"+15550000002"}}, "another SMSC peer has the Diameter identity SMSC-A.example.org"},
			"shared endpoint":       {SMSCPeerParams{DiameterIdentity: "smsc-b.example.org", Address: "2001:db8::10", Port: 3869, ServiceCentres: []string{"+15550000002"}}, "another SMSC peer has the address [2001:db8::10]:3869"},
		}

		for name, tc := range cases {
			code, resp, err := createSMSCPeer(env.Server.URL, client, token, &tc.params)
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}

			if code != http.StatusConflict || resp.Error != tc.reason {
				t.Fatalf("%s: got %d %q, want 409 %q", name, code, resp.Error, tc.reason)
			}
		}

		if got := peers(t); len(got) != 1 {
			t.Fatalf("rejected peers were stored: %+v", got)
		}
	})

	t.Run("a peer is updated", func(t *testing.T) {
		code, resp, err := createSMSCPeer(env.Server.URL, client, token, &SMSCPeerParams{DiameterIdentity: "smsc-b.example.org", Address: "2001:db8::10", Port: 3870, ServiceCentres: []string{"+15550000002"}})
		if err != nil || code != http.StatusCreated {
			t.Fatalf("create: code=%d err=%v (%q)", code, err, resp.Error)
		}

		id := resp.Result.ID

		code, upd, err := updateSMSCPeer(env.Server.URL, client, token, id, &SMSCPeerParams{DiameterIdentity: "smsc-192-0-2-12.example.org", Address: "192.0.2.12", Port: 3868, ServiceCentres: []string{"+15550000003"}})
		if err != nil || code != http.StatusOK {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, upd.Error)
		}

		got := peers(t)[1]
		if got.ID != id || got.DiameterIdentity != "smsc-192-0-2-12.example.org" || got.Address != "192.0.2.12" || got.Port != 3868 || !slices.Equal(got.ServiceCentres, []string{"+15550000003"}) {
			t.Fatalf("peer = %+v", got)
		}

		code, upd, err = updateSMSCPeer(env.Server.URL, client, token, id, &SMSCPeerParams{DiameterIdentity: "smsc-192-0-2-12.example.org", Address: "192.0.2.12", ServiceCentres: []string{"+15550000003"}})
		if err != nil || code != http.StatusBadRequest {
			t.Fatalf("update without a port: code=%d err=%v (%q)", code, err, upd.Error)
		}

		code, upd, err = updateSMSCPeer(env.Server.URL, client, token, id, &SMSCPeerParams{DiameterIdentity: "smsc-192-0-2-12.example.org", Address: "192.0.2.12", Port: 3868, ServiceCentres: []string{"+15550000000"}})
		if err != nil || code != http.StatusConflict {
			t.Fatalf("update to a served service centre: code=%d err=%v (%q)", code, err, upd.Error)
		}

		code, upd, err = updateSMSCPeer(env.Server.URL, client, token, "missing", &SMSCPeerParams{DiameterIdentity: "smsc-192-0-2-13.example.org", Address: "192.0.2.13", Port: 3868, ServiceCentres: []string{"+15550000004"}})
		if err != nil || code != http.StatusNotFound {
			t.Fatalf("update of a missing peer: code=%d err=%v (%q)", code, err, upd.Error)
		}
	})

	t.Run("the last peer can be deleted", func(t *testing.T) {
		for _, p := range peers(t) {
			code, resp, err := deleteSMSCPeer(env.Server.URL, client, token, p.ID)
			if err != nil || code != http.StatusOK {
				t.Fatalf("delete: code=%d err=%v (%q)", code, err, resp.Error)
			}
		}

		if got := peers(t); len(got) != 0 {
			t.Fatalf("peers = %+v, want none", got)
		}

		code, resp, err := deleteSMSCPeer(env.Server.URL, client, token, "missing")
		if err != nil || code != http.StatusNotFound {
			t.Fatalf("delete of a missing peer: code=%d err=%v (%q)", code, err, resp.Error)
		}
	})

	t.Run("a peer without an identity takes the one it gives", func(t *testing.T) {
		code, resp, err := createSMSCPeer(env.Server.URL, client, token, &SMSCPeerParams{Address: "192.0.2.20", ServiceCentres: []string{"+15550000005"}})
		if err != nil || code != http.StatusCreated || resp.Result.DiameterIdentity != "" {
			t.Fatalf("create: code=%d err=%v (%+v)", code, err, resp)
		}

		code, resp, err = createSMSCPeer(env.Server.URL, client, token, &SMSCPeerParams{Address: "192.0.2.20", Port: 3869, ServiceCentres: []string{"+15550000006"}})
		if err != nil || code != http.StatusConflict || resp.Error != "another SMSC peer without a Diameter identity has the address 192.0.2.20" {
			t.Fatalf("second peer without an identity on the same address: code=%d err=%v (%q)", code, err, resp.Error)
		}
	})
}
