// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"testing"
)

type SliceDataNetwork struct {
	DataNetwork     string `json:"data_network"`
	NAT             bool   `json:"nat"`
	LocalSwitch     bool   `json:"local_switch"`
	NetworkInstance string `json:"network_instance"`
}

type ListSliceDataNetworksResponse struct {
	Result struct {
		Items []SliceDataNetwork `json:"items"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

type GetSliceDataNetworkResponse struct {
	Result SliceDataNetwork `json:"result"`
	Error  string           `json:"error,omitempty"`
}

func TestSliceDataNetworks(t *testing.T) {
	env, client, token := newAuthedTestEnv(t)
	url := env.Server.URL
	off, on := false, true

	path := func(slice, dn string) string {
		p := url + "/api/v1/slices/" + slice + "/data-networks"
		if dn != "" {
			p += "/" + dn
		}

		return p
	}

	get := func(t *testing.T, slice, dn string) SliceDataNetwork {
		t.Helper()

		code, resp, err := apiDo[GetSliceDataNetworkResponse](client, "GET", path(slice, dn), token, nil)
		if err != nil || code != http.StatusOK {
			t.Fatalf("get %s/%s: %d %v %+v", slice, dn, code, err, resp)
		}

		return resp.Result
	}

	do := func(t *testing.T, method, p string, body any, want int) {
		t.Helper()

		code, resp, err := apiDo[CreatePolicyResponse](client, method, p, token, body)
		if err != nil || code != want {
			t.Fatalf("%s %s: %d %v %+v, want %d", method, p, code, err, resp, want)
		}
	}

	for _, dn := range []*CreateDataNetworkParams{
		{Name: "corp", IPv4Pool: "10.50.0.0/22", DNS: "8.8.8.8", MTU: 1400},
		{Name: "ims", IPv4Pool: "10.52.0.0/22", DNS: "8.8.8.8", MTU: 1400},
	} {
		if code, resp, err := createDataNetwork(url, client, token, dn); err != nil || code != http.StatusCreated {
			t.Fatalf("create %s: %d %v %+v", dn.Name, code, err, resp)
		}
	}

	if code, resp, err := createSlice(url, client, token, &CreateSliceParams{Name: "iot", Sst: 1, Sd: "000001"}); err != nil || code != http.StatusCreated {
		t.Fatalf("create slice iot: %d %v %+v", code, err, resp)
	}

	t.Run("the default slice serves the default data network", func(t *testing.T) {
		if f := get(t, "default", "internet"); !f.NAT || f.LocalSwitch {
			t.Fatalf("default/internet = %+v, want NAT on and local switching off", f)
		}
	})

	t.Run("a policy needs the slice to serve the data network", func(t *testing.T) {
		code, _, err := apiDo[CreatePolicyResponse](client, "POST", url+"/api/v1/policies", token, &CreatePolicyParams{
			Name: "corp-iot", ProfileName: "default", SliceName: "iot", DataNetworkName: "corp",
			SessionAmbrUplink: "10 Mbps", SessionAmbrDownlink: "10 Mbps", Var5qi: 9, Arp: 1,
		})
		if err != nil || code != http.StatusConflict {
			t.Fatalf("create policy on an unserved data network: %d %v, want 409", code, err)
		}
	})

	t.Run("serve with settings", func(t *testing.T) {
		do(t, "POST", path("iot", ""), map[string]any{"data_network": "corp", "nat": false, "local_switch": true}, http.StatusCreated)
		do(t, "POST", path("iot", ""), map[string]any{"data_network": "corp"}, http.StatusConflict)

		if f := get(t, "iot", "corp"); f.NAT || !f.LocalSwitch {
			t.Fatalf("iot/corp = %+v, want NAT off and local switching on", f)
		}
	})

	t.Run("each slice has its own settings", func(t *testing.T) {
		do(t, "POST", path("default", ""), map[string]any{"data_network": "corp"}, http.StatusCreated)

		if f := get(t, "default", "corp"); !f.NAT || f.LocalSwitch {
			t.Fatalf("default/corp = %+v, want the defaults", f)
		}

		code, resp, err := apiDo[ListSliceDataNetworksResponse](client, "GET", path("iot", ""), token, nil)
		if err != nil || code != http.StatusOK || len(resp.Result.Items) != 1 || resp.Result.Items[0].DataNetwork != "corp" {
			t.Fatalf("list iot: %d %v %+v", code, err, resp)
		}
	})

	t.Run("update", func(t *testing.T) {
		do(t, "PUT", path("iot", "corp"), map[string]any{"nat": true}, http.StatusOK)

		if f := get(t, "iot", "corp"); !f.NAT || !f.LocalSwitch {
			t.Fatalf("iot/corp after update = %+v, want NAT on and local switching kept on", f)
		}

		do(t, "PUT", path("iot", "corp"), map[string]any{"local_switch": false}, http.StatusOK)

		if f := get(t, "iot", "corp"); !f.NAT || f.LocalSwitch {
			t.Fatalf("iot/corp after update = %+v, want local switching off", f)
		}
	})

	t.Run("network instance", func(t *testing.T) {
		if f := get(t, "iot", "corp"); f.NetworkInstance != "default" {
			t.Fatalf("iot/corp network instance = %q, want default", f.NetworkInstance)
		}

		do(t, "PUT", path("iot", "corp"), map[string]any{"network_instance": "Bad_Name"}, http.StatusBadRequest)
		do(t, "PUT", path("iot", "corp"), map[string]any{"network_instance": "enterprise"}, http.StatusOK)

		if f := get(t, "iot", "corp"); f.NetworkInstance != "enterprise" || !f.NAT {
			t.Fatalf("iot/corp after update = %+v, want network instance enterprise and NAT kept", f)
		}

		do(t, "PUT", path("iot", "corp"), map[string]any{"network_instance": ""}, http.StatusOK)

		if f := get(t, "iot", "corp"); f.NetworkInstance != "default" {
			t.Fatalf("iot/corp network instance = %q, want default", f.NetworkInstance)
		}
	})

	t.Run("IMS rules", func(t *testing.T) {
		do(t, "POST", path("default", ""), map[string]any{"data_network": "ims", "nat": true}, http.StatusBadRequest)
		do(t, "POST", path("default", ""), map[string]any{"data_network": "ims"}, http.StatusCreated)

		if f := get(t, "default", "ims"); f.NAT || !f.LocalSwitch {
			t.Fatalf("default/ims = %+v, want NAT off and local switching on", f)
		}

		do(t, "PUT", path("default", "ims"), map[string]any{"local_switch": off}, http.StatusBadRequest)
		do(t, "PUT", path("default", "ims"), map[string]any{"nat": on}, http.StatusBadRequest)
	})

	t.Run("stop serving", func(t *testing.T) {
		if code, resp, err := createPolicy(url, client, token, &CreatePolicyParams{
			Name: "corp-iot", ProfileName: "default", SliceName: "iot", DataNetworkName: "corp",
			SessionAmbrUplink: "10 Mbps", SessionAmbrDownlink: "10 Mbps", Var5qi: 9, Arp: 1,
		}); err != nil || code != http.StatusCreated {
			t.Fatalf("create policy: %d %v %+v", code, err, resp)
		}

		do(t, "DELETE", path("iot", "corp"), nil, http.StatusConflict)

		if code, _, err := deletePolicy(url, client, token, "corp-iot"); err != nil || code != http.StatusOK {
			t.Fatalf("delete policy: %d %v", code, err)
		}

		do(t, "DELETE", path("iot", "corp"), nil, http.StatusOK)
		do(t, "GET", path("iot", "corp"), nil, http.StatusNotFound)
	})

	t.Run("removed endpoints are gone", func(t *testing.T) {
		for _, method := range []string{"GET", "PUT"} {
			for _, p := range []string{"/api/v1/networking/nat", "/api/v1/networking/local-switch"} {
				req, err := http.NewRequestWithContext(t.Context(), method, url+p, nil)
				if err != nil {
					t.Fatal(err)
				}

				req.Header.Set("Authorization", "Bearer "+token)

				res, err := client.Do(req)
				if err != nil {
					t.Fatalf("%s %s: %v", method, p, err)
				}

				_ = res.Body.Close()

				if res.StatusCode != http.StatusNotFound {
					t.Fatalf("%s %s: %d, want 404", method, p, res.StatusCode)
				}
			}
		}
	})
}
