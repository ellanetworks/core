// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"slices"
	"testing"
)

type IMSSettingsParams struct {
	PCSCFAddresses []string `json:"pcscfAddresses"`
	HomeDomain     string   `json:"homeDomain"`
	AdvertiseVoice bool     `json:"advertiseVoice"`
	RestrictPeers  bool     `json:"restrictPeers"`
}

type IMSSettingsResult struct {
	PCSCFAddresses    []string `json:"pcscfAddresses"`
	VoiceStatus       string   `json:"voiceStatus"`
	HomeDomain        string   `json:"homeDomain"`
	DerivedHomeDomain string   `json:"derivedHomeDomain"`
	AdvertiseVoice    bool     `json:"advertiseVoice"`
	RestrictPeers     bool     `json:"restrictPeers"`
}

type GetIMSSettingsResponse struct {
	Result IMSSettingsResult `json:"result"`
	Error  string            `json:"error,omitempty"`
}

type UpdateOperatorIMSResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func updateIMSSettings(url string, client *http.Client, token string, data *IMSSettingsParams) (int, *UpdateOperatorIMSResponse, error) {
	return apiDo[UpdateOperatorIMSResponse](client, "PUT", url+"/api/v1/integrations/ims", token, data)
}

func getIMSSettings(url string, client *http.Client, token string) (int, *GetIMSSettingsResponse, error) {
	return apiDo[GetIMSSettingsResponse](client, "GET", url+"/api/v1/integrations/ims", token, nil)
}

func TestUpdateIMSSettings(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	getIMS := func(t *testing.T) IMSSettingsResult {
		t.Helper()

		code, resp, err := getIMSSettings(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("get IMS settings: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("get IMS settings: expected 200, got %d (%q)", code, resp.Error)
		}

		if resp.Result.PCSCFAddresses == nil {
			t.Fatal("pcscfAddresses is null, want a list")
		}

		return resp.Result
	}

	getPCSCFAddresses := func(t *testing.T) []string {
		t.Helper()

		return getIMS(t).PCSCFAddresses
	}

	update := func(t *testing.T, addresses []string) {
		t.Helper()

		code, resp, err := updateIMSSettings(env.Server.URL, client, token, &IMSSettingsParams{PCSCFAddresses: addresses, AdvertiseVoice: true})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("update: expected 200, got %d (%q)", code, resp.Error)
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
		update(t, []string{" PCSCF.ims.example.org ", " 2001:DB8:0::20 "})

		if got := getPCSCFAddresses(t); !slices.Equal(got, []string{"pcscf.ims.example.org", "2001:db8::20"}) {
			t.Fatalf("pcscfAddresses = %v, want the canonical pcscf.ims.example.org and 2001:db8::20", got)
		}

		update(t, []string{" 2001:DB8:0::20 "})

		if got := getPCSCFAddresses(t); !slices.Equal(got, []string{"2001:db8::20"}) {
			t.Fatalf("pcscfAddresses = %v, want the canonical 2001:db8::20", got)
		}
	})

	t.Run("invalid lists are rejected", func(t *testing.T) {
		cases := map[string][]string{
			"bad hostname":       {"p_cscf.example.org"},
			"duplicate hostname": {"pcscf.example.org", "PCSCF.example.org"},
			"duplicate":          {"192.0.2.20", "192.0.2.20"},
			"six ipv4":           {"192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4", "192.0.2.5", "192.0.2.6"},
		}

		for name, addresses := range cases {
			code, resp, err := updateIMSSettings(env.Server.URL, client, token, &IMSSettingsParams{PCSCFAddresses: addresses})
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

	t.Run("the home domain defaults to the derived domain", func(t *testing.T) {
		ims := getIMS(t)
		if ims.HomeDomain != "" || ims.DerivedHomeDomain != "ims.mnc001.mcc001.3gppnetwork.org" {
			t.Fatalf("homeDomain = %q, derivedHomeDomain = %q", ims.HomeDomain, ims.DerivedHomeDomain)
		}

		if ims.RestrictPeers {
			t.Fatal("restrictPeers = true, want false by default")
		}
	})

	t.Run("the home domain is set and reset", func(t *testing.T) {
		code, resp, err := updateIMSSettings(env.Server.URL, client, token, &IMSSettingsParams{PCSCFAddresses: []string{}, HomeDomain: " IMS.Example.COM ", AdvertiseVoice: true})
		if err != nil || code != http.StatusOK {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		if got := getIMS(t).HomeDomain; got != "ims.example.com" {
			t.Fatalf("homeDomain = %q, want ims.example.com", got)
		}

		code, resp, err = updateIMSSettings(env.Server.URL, client, token, &IMSSettingsParams{PCSCFAddresses: []string{}, AdvertiseVoice: true})
		if err != nil || code != http.StatusOK {
			t.Fatalf("reset: code=%d err=%v (%q)", code, err, resp.Error)
		}

		if got := getIMS(t).HomeDomain; got != "" {
			t.Fatalf("homeDomain = %q, want it reset", got)
		}
	})

	t.Run("invalid home domains are rejected", func(t *testing.T) {
		for _, domain := range []string{"localhost", "192.0.2.1", "ims_example.com", "-ims.example.com", "epc.mnc001.mcc001.3gppnetwork.org", "ims.mnc093.mcc208.3gppnetwork.org"} {
			code, resp, err := updateIMSSettings(env.Server.URL, client, token, &IMSSettingsParams{PCSCFAddresses: []string{}, HomeDomain: domain})
			if err != nil {
				t.Fatalf("%s: %s", domain, err)
			}

			if code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d (%q)", domain, code, resp.Error)
			}
		}
	})

	t.Run("the derived home domain is stored as the default", func(t *testing.T) {
		code, resp, err := updateIMSSettings(env.Server.URL, client, token, &IMSSettingsParams{PCSCFAddresses: []string{}, HomeDomain: "ims.mnc001.mcc001.3gppnetwork.org", AdvertiseVoice: true})
		if err != nil || code != http.StatusOK {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		if got := getIMS(t).HomeDomain; got != "" {
			t.Fatalf("homeDomain = %q, want empty", got)
		}
	})

	t.Run("every field is required", func(t *testing.T) {
		for name, body := range map[string]map[string]any{
			"no home domain":        {"pcscfAddresses": []string{}, "advertiseVoice": true, "restrictPeers": false},
			"no advertiseVoice":     {"pcscfAddresses": []string{}, "homeDomain": "", "restrictPeers": false},
			"null advertiseVoice":   {"pcscfAddresses": []string{}, "homeDomain": "", "advertiseVoice": nil, "restrictPeers": false},
			"string advertiseVoice": {"pcscfAddresses": []string{}, "homeDomain": "", "advertiseVoice": "yes", "restrictPeers": false},
			"no restrictPeers":      {"pcscfAddresses": []string{}, "homeDomain": "", "advertiseVoice": true},
			"null restrictPeers":    {"pcscfAddresses": []string{}, "homeDomain": "", "advertiseVoice": true, "restrictPeers": nil},
			"string restrictPeers":  {"pcscfAddresses": []string{}, "homeDomain": "", "advertiseVoice": true, "restrictPeers": "yes"},
		} {
			code, resp, err := apiDo[UpdateOperatorIMSResponse](client, "PUT", env.Server.URL+"/api/v1/integrations/ims", token, body)
			if err != nil {
				t.Fatal(err)
			}

			if code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d (%q)", name, code, resp.Error)
			}
		}
	})

	t.Run("IMS voice is advertised by default and can be turned off", func(t *testing.T) {
		update(t, []string{"192.0.2.20"})

		if !getIMS(t).AdvertiseVoice {
			t.Fatal("advertiseVoice = false, want true by default")
		}

		code, resp, err := updateIMSSettings(env.Server.URL, client, token, &IMSSettingsParams{PCSCFAddresses: []string{"192.0.2.21"}, HomeDomain: "ims.example.com", AdvertiseVoice: false})
		if err != nil || code != http.StatusOK {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		ims := getIMS(t)
		if ims.AdvertiseVoice || ims.HomeDomain != "ims.example.com" || !slices.Equal(ims.PCSCFAddresses, []string{"192.0.2.21"}) {
			t.Fatalf("IMS settings = %+v, want every field updated together", ims)
		}
	})

	t.Run("peers can be restricted to the allowed IMS peers", func(t *testing.T) {
		code, resp, err := updateIMSSettings(env.Server.URL, client, token, &IMSSettingsParams{PCSCFAddresses: []string{"192.0.2.21"}, HomeDomain: "ims.example.com", RestrictPeers: true})
		if err != nil || code != http.StatusOK {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		ims := getIMS(t)
		if !ims.RestrictPeers || ims.AdvertiseVoice || ims.HomeDomain != "ims.example.com" {
			t.Fatalf("IMS settings = %+v, want peers restricted and the rest unchanged", ims)
		}
	})
}
