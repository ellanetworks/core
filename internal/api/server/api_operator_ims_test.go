// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"slices"
	"testing"
)

type UpdateOperatorIMSParams struct {
	PCSCFAddresses []string `json:"pcscfAddresses"`
}

type UpdateOperatorIMSResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func updateOperatorIMS(url string, client *http.Client, token string, data *UpdateOperatorIMSParams) (int, *UpdateOperatorIMSResponse, error) {
	return apiDo[UpdateOperatorIMSResponse](client, "PUT", url+"/api/v1/operator/ims", token, data)
}

func TestUpdateOperatorIMS(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	getIMS := func(t *testing.T) GetOperatorIMSResponseResult {
		t.Helper()

		code, resp, err := getOperator(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("get operator: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("get operator: expected 200, got %d (%q)", code, resp.Error)
		}

		if resp.Result.IMS.PCSCFAddresses == nil {
			t.Fatal("pcscfAddresses is null, want a list")
		}

		return resp.Result.IMS
	}

	getPCSCFAddresses := func(t *testing.T) []string {
		t.Helper()

		return getIMS(t).PCSCFAddresses
	}

	update := func(t *testing.T, addresses []string) {
		t.Helper()

		code, resp, err := updateOperatorIMS(env.Server.URL, client, token, &UpdateOperatorIMSParams{PCSCFAddresses: addresses})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusCreated {
			t.Fatalf("update: expected 201, got %d (%q)", code, resp.Error)
		}
	}

	t.Run("no P-CSCF addresses by default", func(t *testing.T) {
		ims := getIMS(t)
		if len(ims.PCSCFAddresses) != 0 {
			t.Fatalf("pcscfAddresses = %v, want none", ims.PCSCFAddresses)
		}

		if ims.VoiceStatus != "no_pcscf" {
			t.Fatalf("voiceStatus = %q, want no_pcscf", ims.VoiceStatus)
		}
	})

	t.Run("addresses keep their order", func(t *testing.T) {
		want := []string{"2001:db8::20", "192.0.2.20", "192.0.2.21"}
		update(t, want)

		ims := getIMS(t)
		if !slices.Equal(ims.PCSCFAddresses, want) {
			t.Fatalf("pcscfAddresses = %v, want %v", ims.PCSCFAddresses, want)
		}

		if ims.VoiceStatus != "no_ims_data_network" {
			t.Fatalf("voiceStatus = %q, want no_ims_data_network", ims.VoiceStatus)
		}
	})

	t.Run("addresses are stored in canonical form", func(t *testing.T) {
		update(t, []string{" 2001:DB8:0::20 "})

		if got := getPCSCFAddresses(t); !slices.Equal(got, []string{"2001:db8::20"}) {
			t.Fatalf("pcscfAddresses = %v, want the canonical 2001:db8::20", got)
		}
	})

	t.Run("invalid lists are rejected", func(t *testing.T) {
		cases := map[string][]string{
			"hostname":  {"pcscf.example.org"},
			"duplicate": {"192.0.2.20", "192.0.2.20"},
			"four ipv4": {"192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4"},
		}

		for name, addresses := range cases {
			code, resp, err := updateOperatorIMS(env.Server.URL, client, token, &UpdateOperatorIMSParams{PCSCFAddresses: addresses})
			if err != nil {
				t.Fatalf("%s: %s", name, err)
			}

			if code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d (%q)", name, code, resp.Error)
			}
		}

		if got := getPCSCFAddresses(t); !slices.Equal(got, []string{"2001:db8::20"}) {
			t.Fatalf("rejected updates changed the addresses: %v", got)
		}
	})

	t.Run("an empty list clears the addresses", func(t *testing.T) {
		update(t, []string{})

		if got := getPCSCFAddresses(t); len(got) != 0 {
			t.Fatalf("pcscfAddresses = %v, want none", got)
		}
	})
}
