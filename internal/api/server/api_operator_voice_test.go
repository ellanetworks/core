// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"slices"
	"testing"
)

type UpdateOperatorVoiceParams struct {
	PCSCFAddresses []string `json:"pcscfAddresses"`
}

type UpdateOperatorVoiceResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func updateOperatorVoice(url string, client *http.Client, token string, data *UpdateOperatorVoiceParams) (int, *UpdateOperatorVoiceResponse, error) {
	return apiDo[UpdateOperatorVoiceResponse](client, "PUT", url+"/api/v1/operator/voice", token, data)
}

func TestUpdateOperatorVoice(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	getPCSCFAddresses := func(t *testing.T) []string {
		t.Helper()

		code, resp, err := getOperator(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("get operator: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("get operator: expected 200, got %d (%q)", code, resp.Error)
		}

		if resp.Result.Voice.PCSCFAddresses == nil {
			t.Fatal("pcscfAddresses is null, want a list")
		}

		return resp.Result.Voice.PCSCFAddresses
	}

	update := func(t *testing.T, addresses []string) {
		t.Helper()

		code, resp, err := updateOperatorVoice(env.Server.URL, client, token, &UpdateOperatorVoiceParams{PCSCFAddresses: addresses})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusCreated {
			t.Fatalf("update: expected 201, got %d (%q)", code, resp.Error)
		}
	}

	t.Run("no P-CSCF addresses by default", func(t *testing.T) {
		if got := getPCSCFAddresses(t); len(got) != 0 {
			t.Fatalf("pcscfAddresses = %v, want none", got)
		}
	})

	t.Run("addresses keep their order", func(t *testing.T) {
		want := []string{"2001:db8::20", "192.0.2.20", "192.0.2.21"}
		update(t, want)

		if got := getPCSCFAddresses(t); !slices.Equal(got, want) {
			t.Fatalf("pcscfAddresses = %v, want %v", got, want)
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
			"hostname":         {"pcscf.example.org"},
			"duplicate":        {"192.0.2.20", "192.0.2.20"},
			"four ipv4":        {"192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4"},
			"four ipv6":        {"2001:db8::1", "2001:db8::2", "2001:db8::3", "2001:db8::4"},
			"unspecified":      {"0.0.0.0"},
			"loopback":         {"127.0.0.1"},
			"multicast":        {"ff02::1"},
			"zone":             {"fe80::1%eth0"},
			"ipv4-mapped ipv6": {"::ffff:192.0.2.20"},
		}

		for name, addresses := range cases {
			code, resp, err := updateOperatorVoice(env.Server.URL, client, token, &UpdateOperatorVoiceParams{PCSCFAddresses: addresses})
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
