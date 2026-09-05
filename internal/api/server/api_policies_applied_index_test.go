// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func policyWriteAppliedIndex(t *testing.T, client *http.Client, method, url, token string, payload any) string {
	t.Helper()

	var body bytes.Buffer

	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode payload: %v", err)
		}
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, &body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("%s %s: unexpected status %d", method, url, resp.StatusCode)
	}

	return resp.Header.Get("X-Ella-Applied-Index")
}

func TestPolicyWritesStampTheAppliedIndex(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)

	code, _, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{
		Name: "dn-idx", MTU: 1500, IPv4Pool: "10.60.0.0/24", DNS: "8.8.8.8",
	})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create data network: status %d, err %v", code, err)
	}

	code, _, err = createProfile(env.Server.URL, client, token, &CreateProfileParams{
		Name: "profile-idx", UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "200 Mbps",
	})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create profile: status %d, err %v", code, err)
	}

	for _, tc := range []struct {
		name    string
		method  string
		path    string
		payload any
	}{
		{
			name:   "create",
			method: "POST",
			path:   "/api/v1/policies",
			payload: &CreatePolicyParams{
				Name: "policy-idx", ProfileName: "profile-idx", SliceName: "default",
				DataNetworkName: "dn-idx", SessionAmbrUplink: "100 Mbps",
				SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 6,
			},
		},
		{
			name:   "update",
			method: "PUT",
			path:   "/api/v1/policies/policy-idx",
			payload: &UpdatePolicyParams{
				ProfileName: "profile-idx", SliceName: "default",
				DataNetworkName: "dn-idx", SessionAmbrUplink: "100 Mbps",
				SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 6,
			},
		},
		{
			name:   "delete",
			method: "DELETE",
			path:   "/api/v1/policies/policy-idx",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := policyWriteAppliedIndex(t, client, tc.method, env.Server.URL+tc.path, token, tc.payload)
			if got == "" {
				t.Error("policy write did not stamp X-Ella-Applied-Index")
			}
		})
	}
}
