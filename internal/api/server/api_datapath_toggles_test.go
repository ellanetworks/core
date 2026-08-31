// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"testing"
)

type ToggleParams struct {
	Enabled bool `json:"enabled"`
}

type ToggleResponseResult struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

type ToggleResponse struct {
	Result ToggleResponseResult `json:"result"`
	Error  string               `json:"error,omitempty"`
}

var datapathToggles = []struct {
	name       string
	path       string
	updatedMsg string
}{
	{
		name:       "NAT",
		path:       "/api/v1/networking/nat",
		updatedMsg: "NAT settings updated successfully",
	},
	{
		name:       "flow accounting",
		path:       "/api/v1/networking/flow-accounting",
		updatedMsg: "Flow accounting settings updated successfully",
	},
}

func assertToggleEnabled(t *testing.T, url string, client *http.Client, token, path string, want bool) {
	t.Helper()

	statusCode, response, err := apiDo[ToggleResponse](client, "GET", url+path, token, nil)
	if err != nil {
		t.Fatalf("couldn't get %s: %s", path, err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Error != "" {
		t.Fatalf("expected no error, got %s", response.Error)
	}

	if response.Result.Enabled != want {
		t.Fatalf("enabled = %v, want %v", response.Result.Enabled, want)
	}
}

func TestAPIDatapathTogglesEndToEnd(t *testing.T) {
	for _, tg := range datapathToggles {
		t.Run(tg.name, func(t *testing.T) {
			env, client, token := newAuthedTestEnv(t)

			assertToggleEnabled(t, env.Server.URL, client, token, tg.path, true)

			statusCode, response, err := apiDo[ToggleResponse](client, "PUT", env.Server.URL+tg.path, token, ToggleParams{Enabled: false})
			if err != nil {
				t.Fatalf("couldn't update %s: %s", tg.name, err)
			}

			if statusCode != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
			}

			if response.Error != "" {
				t.Fatalf("expected no error, got %s", response.Error)
			}

			if response.Result.Message != tg.updatedMsg {
				t.Fatalf("message = %q, want %q", response.Result.Message, tg.updatedMsg)
			}

			assertToggleEnabled(t, env.Server.URL, client, token, tg.path, false)
		})
	}
}
