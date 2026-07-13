// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	framedDN      = "framed-dn"
	framedSlice   = "framed-slice"
	framedProfile = "framed-profile"
	framedIPv4    = "10.60.0.0/24"
	framedIPv6    = "2001:db8:6000::/48"
)

type framedRouteItem struct {
	IMSI string   `json:"imsi"`
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

type framedListResult struct {
	Items      []framedRouteItem `json:"items"`
	TotalCount int               `json:"total_count"`
}

type framedListResponse struct {
	Result framedListResult `json:"result"`
	Error  string           `json:"error,omitempty"`
}

func doFramedMessageRequest(client *http.Client, method, path, token, body string) (int, *messageResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var out messageResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &out, nil
}

func listFramedRoutes(url string, client *http.Client, token string) (int, *framedListResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/data-networks/"+framedDN+"/framed-routes", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var out framedListResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &out, nil
}

func createFramedRoute(url string, client *http.Client, token, dn, imsi string, ipv4, ipv6 []string) (int, *messageResponse, error) {
	body, _ := json.Marshal(map[string]any{"imsi": imsi, "ipv4": ipv4, "ipv6": ipv6})

	return doFramedMessageRequest(client, "POST", url+"/api/v1/networking/data-networks/"+dn+"/framed-routes", token, string(body))
}

func updateFramedRoute(url string, client *http.Client, token, dn, imsi string, ipv4, ipv6 []string) (int, *messageResponse, error) {
	body, _ := json.Marshal(map[string]any{"ipv4": ipv4, "ipv6": ipv6})

	return doFramedMessageRequest(client, "PUT", url+"/api/v1/networking/data-networks/"+dn+"/framed-routes/"+imsi, token, string(body))
}

func deleteFramedRoute(url string, client *http.Client, token, dn, imsi string) (int, *messageResponse, error) {
	return doFramedMessageRequest(client, "DELETE", url+"/api/v1/networking/data-networks/"+dn+"/framed-routes/"+imsi, token, "")
}

func TestAPIFramedRoutesEndToEnd(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)
	url := env.Server.URL

	token, err := initializeAndRefresh(url, client)
	if err != nil {
		t.Fatalf("couldn't create first user and login: %s", err)
	}

	mustCreate := func(name string, code int, callErr error) {
		t.Helper()

		if callErr != nil {
			t.Fatalf("%s: %s", name, callErr)
		}

		if code != http.StatusCreated {
			t.Fatalf("%s: expected 201, got %d", name, code)
		}
	}

	sc, _, err := createSlice(url, client, token, &CreateSliceParams{Name: framedSlice, Sst: 1})
	mustCreate("createSlice", sc, err)

	sc, _, err = createDataNetwork(url, client, token, &CreateDataNetworkParams{
		Name: framedDN, IPv4Pool: framedIPv4, IPv6Pool: framedIPv6, DNS: DNS, MTU: MTU,
	})
	mustCreate("createDataNetwork", sc, err)

	sc, _, err = createProfile(url, client, token, &CreateProfileParams{
		Name: framedProfile, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "200 Mbps",
	})
	mustCreate("createProfile", sc, err)

	sc, _, err = createPolicy(url, client, token, &CreatePolicyParams{
		Name: "framed-policy", ProfileName: framedProfile, SliceName: framedSlice,
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "200 Mbps",
		Var5qi: 9, Arp: 1, DataNetworkName: framedDN,
	})
	mustCreate("createPolicy", sc, err)

	sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{
		Imsi: Imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: framedProfile,
	})
	mustCreate("createSubscriber", sc, err)

	// A second subscriber on the same profile (also bound to framedDN), for the
	// validation cases so the primary subscriber's set stays intact.
	valIMSI := "001010100008888"
	sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{
		Imsi: valIMSI, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: framedProfile,
	})
	mustCreate("createSubscriber(val)", sc, err)

	// An orphan subscriber whose profile binds a different data network.
	orphanIMSI := "001010100009999"
	sc, _, err = createDataNetwork(url, client, token, &CreateDataNetworkParams{Name: "framed-other-dn", IPv4Pool: "10.77.0.0/24", DNS: DNS, MTU: MTU})
	mustCreate("createDataNetwork(other)", sc, err)
	sc, _, err = createProfile(url, client, token, &CreateProfileParams{Name: "framed-orphan-profile", UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps"})
	mustCreate("createProfile(orphan)", sc, err)
	sc, _, err = createPolicy(url, client, token, &CreatePolicyParams{
		Name: "framed-orphan-policy", ProfileName: "framed-orphan-profile", SliceName: framedSlice,
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkName: "framed-other-dn",
	})
	mustCreate("createPolicy(orphan)", sc, err)
	sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{Imsi: orphanIMSI, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: "framed-orphan-profile"})
	mustCreate("createSubscriber(orphan)", sc, err)

	assertCode := func(t *testing.T, want, got int, resp *messageResponse) {
		t.Helper()

		if got != want {
			msg := ""
			if resp != nil {
				msg = resp.Error
			}

			t.Fatalf("expected %d, got %d (%s)", want, got, msg)
		}
	}

	t.Run("reverse NAT guard: enabling NAT is rejected while framed routes exist", func(t *testing.T) {
		// First enable then disable while empty succeeds.
		if code, _, err := updateNATInfo(url, client, token, true); err != nil || code != http.StatusOK {
			t.Fatalf("enable NAT (empty): expected 200, got %d (%v)", code, err)
		}

		if code, _, err := updateNATInfo(url, client, token, false); err != nil || code != http.StatusOK {
			t.Fatalf("disable NAT: expected 200, got %d (%v)", code, err)
		}
	})

	t.Run("create happy path", func(t *testing.T) {
		code, resp, err := createFramedRoute(url, client, token, framedDN, Imsi, []string{"192.168.60.0/24"}, []string{"2001:db8:7000::/48"})
		if err != nil {
			t.Fatalf("createFramedRoute: %s", err)
		}

		assertCode(t, http.StatusCreated, code, resp)
	})

	t.Run("list reflects the set grouped by subscriber", func(t *testing.T) {
		code, resp, err := listFramedRoutes(url, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%v)", code, err)
		}

		if len(resp.Result.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(resp.Result.Items))
		}

		it := resp.Result.Items[0]
		if it.IMSI != Imsi || len(it.IPv4) != 1 || it.IPv4[0] != "192.168.60.0/24" || len(it.IPv6) != 1 {
			t.Fatalf("unexpected item %+v", it)
		}
	})

	t.Run("create again for the same subscriber is 409", func(t *testing.T) {
		code, resp, _ := createFramedRoute(url, client, token, framedDN, Imsi, []string{"192.168.99.0/24"}, nil)
		assertCode(t, http.StatusConflict, code, resp)
	})

	t.Run("normalization: host bits masked in storage and listing", func(t *testing.T) {
		code, resp, _ := createFramedRoute(url, client, token, framedDN, valIMSI, []string{"172.16.5.9/24"}, nil)
		assertCode(t, http.StatusCreated, code, resp)

		code, list, err := listFramedRoutes(url, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("list: %d %v", code, err)
		}

		found := false

		for _, it := range list.Result.Items {
			if it.IMSI == valIMSI {
				if len(it.IPv4) != 1 || it.IPv4[0] != "172.16.5.0/24" {
					t.Fatalf("expected normalized 172.16.5.0/24, got %+v", it.IPv4)
				}

				found = true
			}
		}

		if !found {
			t.Fatalf("valIMSI routes not listed")
		}

		// Clean up so valIMSI is route-free for the validation cases below.
		if code, resp, _ := deleteFramedRoute(url, client, token, framedDN, valIMSI); code != http.StatusOK {
			t.Fatalf("cleanup delete: %d (%s)", code, resp.Error)
		}
	})

	// These all fail before any write, so valIMSI stays route-free.
	t.Run("validation errors", func(t *testing.T) {
		cases := []struct {
			name string
			ipv4 []string
			ipv6 []string
			want int
		}{
			{"cap exceeded", []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24", "10.0.4.0/24", "10.0.5.0/24", "10.0.6.0/24", "10.0.7.0/24", "10.0.8.0/24"}, nil, http.StatusBadRequest},
			{"duplicate in request", []string{"192.168.70.0/24", "192.168.70.0/24"}, nil, http.StatusBadRequest},
			{"overlapping in request", []string{"10.70.0.0/24", "10.70.0.128/25"}, nil, http.StatusBadRequest},
			{"malformed CIDR", []string{"not-a-cidr"}, nil, http.StatusBadRequest},
			{"wrong family", []string{"2001:db8::/48"}, nil, http.StatusBadRequest},
			{"default route", []string{"0.0.0.0/0"}, nil, http.StatusBadRequest},
			{"reserved range", []string{"127.0.0.0/8"}, nil, http.StatusBadRequest},
			{"overlap UE pool", []string{"10.60.0.0/24"}, nil, http.StatusConflict},
			{"overlap another subscriber framed route", []string{"192.168.60.128/25"}, nil, http.StatusConflict},
			{"overlap UE IPv6 pool", nil, []string{"2001:db8:6000:1::/64"}, http.StatusConflict},
			{"overlap another subscriber IPv6 framed route", nil, []string{"2001:db8:7000:5::/64"}, http.StatusConflict},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				code, resp, _ := createFramedRoute(url, client, token, framedDN, valIMSI, tc.ipv4, tc.ipv6)
				assertCode(t, tc.want, code, resp)
			})
		}
	})

	t.Run("profile not bound to data network is 409", func(t *testing.T) {
		code, resp, _ := createFramedRoute(url, client, token, framedDN, orphanIMSI, []string{"192.168.80.0/24"}, nil)
		assertCode(t, http.StatusConflict, code, resp)
	})

	t.Run("framed write rejected while NAT is enabled", func(t *testing.T) {
		if code, _, err := updateNATInfo(url, client, token, true); err != nil {
			t.Fatalf("enable NAT: %v", err)
		} else if code != http.StatusConflict {
			// NAT enable must be rejected because Imsi holds framed routes.
			t.Fatalf("enable NAT with framed routes present: expected 409, got %d", code)
		}
	})

	t.Run("reverse guard: route overlapping a framed route is 409", func(t *testing.T) {
		code, resp, err := createRoute(url, client, token, &CreateRouteParams{
			Destination: "192.168.60.128/25", Gateway: "10.60.0.1", Interface: "n6", Metric: 100,
		})
		if err != nil {
			t.Fatalf("createRoute: %s", err)
		}

		if code != http.StatusConflict {
			t.Fatalf("expected 409, got %d (%v)", code, resp)
		}
	})

	t.Run("reverse guard: data network pool overlapping a framed route is rejected", func(t *testing.T) {
		code, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{
			Name: "framed-overlap-dn", IPv4Pool: "192.168.60.0/25", DNS: DNS, MTU: MTU,
		})
		if err != nil {
			t.Fatalf("createDataNetwork: %s", err)
		}

		if code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", code)
		}
	})

	t.Run("update replaces the set", func(t *testing.T) {
		code, resp, _ := updateFramedRoute(url, client, token, framedDN, Imsi, []string{"192.168.62.0/24"}, nil)
		assertCode(t, http.StatusOK, code, resp)

		code, list, err := listFramedRoutes(url, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("list: %d %v", code, err)
		}

		for _, it := range list.Result.Items {
			if it.IMSI == Imsi {
				if len(it.IPv4) != 1 || it.IPv4[0] != "192.168.62.0/24" || len(it.IPv6) != 0 {
					t.Fatalf("expected replaced set [192.168.62.0/24], got %+v / %+v", it.IPv4, it.IPv6)
				}
			}
		}
	})

	t.Run("update keeping an own prefix does not self-conflict", func(t *testing.T) {
		// Imsi currently holds 192.168.62.0/24. A replace that keeps it must not
		// conflict with the set it replaces (own-pair exclusion in overlap check).
		code, resp, _ := updateFramedRoute(url, client, token, framedDN, Imsi, []string{"192.168.62.0/24", "192.168.63.0/24"}, nil)
		assertCode(t, http.StatusOK, code, resp)

		code, list, err := listFramedRoutes(url, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("list: %d %v", code, err)
		}

		for _, it := range list.Result.Items {
			if it.IMSI == Imsi && len(it.IPv4) != 2 {
				t.Fatalf("expected 2 routes after self-inclusive update, got %+v", it.IPv4)
			}
		}
	})

	t.Run("update for a subscriber with no set is 404", func(t *testing.T) {
		code, resp, _ := updateFramedRoute(url, client, token, framedDN, valIMSI, []string{"192.168.90.0/24"}, nil)
		assertCode(t, http.StatusNotFound, code, resp)
	})

	t.Run("delete then list is empty for the subscriber", func(t *testing.T) {
		code, resp, _ := deleteFramedRoute(url, client, token, framedDN, Imsi)
		assertCode(t, http.StatusOK, code, resp)

		code, list, err := listFramedRoutes(url, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("list: %d %v", code, err)
		}

		if list.Result.TotalCount != 0 {
			t.Fatalf("expected no framed routes, got %d", list.Result.TotalCount)
		}
	})

	t.Run("delete for a subscriber with no set is 404", func(t *testing.T) {
		code, resp, _ := deleteFramedRoute(url, client, token, framedDN, Imsi)
		assertCode(t, http.StatusNotFound, code, resp)
	})

	t.Run("unknown data network is 404", func(t *testing.T) {
		code, resp, _ := createFramedRoute(url, client, token, "no-such-dn", Imsi, []string{"192.168.95.0/24"}, nil)
		assertCode(t, http.StatusNotFound, code, resp)
	})

	t.Run("unknown subscriber is 404", func(t *testing.T) {
		code, resp, _ := createFramedRoute(url, client, token, framedDN, "001010100000000", []string{"192.168.96.0/24"}, nil)
		assertCode(t, http.StatusNotFound, code, resp)
	})

	t.Run("create at the per-family cap (8) succeeds", func(t *testing.T) {
		capIMSI := "001010100007777"

		sc, _, err := createSubscriber(url, client, token, &CreateSubscriberParams{
			Imsi: capIMSI, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: framedProfile,
		})
		if err != nil || sc != http.StatusCreated {
			t.Fatalf("createSubscriber(cap): %d %v", sc, err)
		}

		eight := []string{
			"172.20.0.0/24", "172.20.1.0/24", "172.20.2.0/24", "172.20.3.0/24",
			"172.20.4.0/24", "172.20.5.0/24", "172.20.6.0/24", "172.20.7.0/24",
		}

		code, resp, _ := createFramedRoute(url, client, token, framedDN, capIMSI, eight, nil)
		assertCode(t, http.StatusCreated, code, resp)
	})
}
