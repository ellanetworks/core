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
	staticDN      = "static-dn"
	staticSlice   = "static-slice"
	staticProfile = "static-profile"
	staticIPv4    = "10.99.0.0/24"
	staticIPv6    = "2001:db8:aaaa::/48"
)

type staticIPItem struct {
	IMSI        string `json:"imsi"`
	DataNetwork string `json:"data_network"`
	IPVersion   string `json:"ip_version"`
	Address     string `json:"address"`
	Status      string `json:"status"`
	SessionID   *int   `json:"session_id"`
}

type listStaticIPsResult struct {
	Items      []staticIPItem `json:"items"`
	TotalCount int            `json:"total_count"`
}

type listStaticIPsResponse struct {
	Result listStaticIPsResult `json:"result"`
	Error  string              `json:"error,omitempty"`
}

type messageResponse struct {
	Result struct {
		Message string `json:"message"`
	} `json:"result"`
	Error string `json:"error,omitempty"`
}

func listStaticIps(url string, client *http.Client, token string) (int, *listStaticIPsResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/networking/data-networks/"+staticDN+"/static-ips", nil)
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

	var out listStaticIPsResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &out, nil
}

func createStaticIp(url string, client *http.Client, token, dn, imsi, address string) (int, *messageResponse, error) {
	body, _ := json.Marshal(map[string]string{"imsi": imsi, "address": address})

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/networking/data-networks/"+dn+"/static-ips", strings.NewReader(string(body)))
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

func updateStaticIp(url string, client *http.Client, token, dn, imsi, ipVersion, address string) (int, *messageResponse, error) {
	body, _ := json.Marshal(map[string]string{"address": address})

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/networking/data-networks/"+dn+"/static-ips/"+imsi+"/"+ipVersion, strings.NewReader(string(body)))
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

func deleteStaticIp(url string, client *http.Client, token, dn, imsi, ipVersion string) (int, *messageResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/networking/data-networks/"+dn+"/static-ips/"+imsi+"/"+ipVersion, nil)
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

func TestAPIStaticIPsEndToEnd(t *testing.T) {
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

	// Provision the profile → slice → data network → policy → subscriber chain.
	mustCreate := func(name string, code int, resErr string, callErr error) {
		t.Helper()

		if callErr != nil {
			t.Fatalf("%s: %s", name, callErr)
		}

		if code != http.StatusCreated {
			t.Fatalf("%s: expected 201, got %d (%s)", name, code, resErr)
		}
	}

	sc, sliceResp, err := createSlice(url, client, token, &CreateSliceParams{Name: staticSlice, Sst: 1})
	mustCreate("createSlice", sc, errStr(sliceResp), err)

	sc, dnResp, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{
		Name: staticDN, IPv4Pool: staticIPv4, IPv6Pool: staticIPv6, DNS: DNS, MTU: MTU,
	})
	mustCreate("createDataNetwork", sc, errStrDN(dnResp), err)

	sc, profResp, err := createProfile(url, client, token, &CreateProfileParams{
		Name: staticProfile, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "200 Mbps",
	})
	mustCreate("createProfile", sc, errStrProf(profResp), err)

	sc, polResp, err := createPolicy(url, client, token, &CreatePolicyParams{
		Name: "static-policy", ProfileName: staticProfile, SliceName: staticSlice,
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "200 Mbps",
		Var5qi: 9, Arp: 1, DataNetworkName: staticDN,
	})
	mustCreate("createPolicy", sc, errStrPol(polResp), err)

	sc, subResp, err := createSubscriber(url, client, token, &CreateSubscriberParams{
		Imsi: Imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: staticProfile,
	})
	mustCreate("createSubscriber", sc, errStrSub(subResp), err)

	// A subscriber whose profile reaches a different data network, so its
	// policy does not bind staticDN (used for the not-bound 409).
	orphanIMSI := "001010100009999"
	sc, _, err = createDataNetwork(url, client, token, &CreateDataNetworkParams{Name: "other-dn", IPv4Pool: "10.77.0.0/24", DNS: DNS, MTU: MTU})
	mustCreate("createDataNetwork(other)", sc, "", err)
	sc, _, err = createProfile(url, client, token, &CreateProfileParams{Name: "orphan-profile", UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps"})
	mustCreate("createProfile(orphan)", sc, "", err)
	sc, _, err = createPolicy(url, client, token, &CreatePolicyParams{
		Name: "orphan-policy", ProfileName: "orphan-profile", SliceName: staticSlice,
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkName: "other-dn",
	})
	mustCreate("createPolicy(orphan)", sc, "", err)
	sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{Imsi: orphanIMSI, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: "orphan-profile"})
	mustCreate("createSubscriber(orphan)", sc, "", err)

	t.Run("create IPv4 happy path", func(t *testing.T) {
		code, resp, err := createStaticIp(url, client, token, staticDN, Imsi, "10.99.0.10")
		if err != nil || code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (%s / %v)", code, resp.Error, err)
		}
	})

	t.Run("list reflects the reservation", func(t *testing.T) {
		code, resp, err := listStaticIps(url, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%v)", code, err)
		}

		if len(resp.Result.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(resp.Result.Items))
		}

		it := resp.Result.Items[0]
		if it.IMSI != Imsi || it.IPVersion != "ipv4" || it.Address != "10.99.0.10" || it.Status != "reserved" {
			t.Fatalf("unexpected item: %+v", it)
		}
	})

	t.Run("duplicate for same imsi+family is 409", func(t *testing.T) {
		code, _, _ := createStaticIp(url, client, token, staticDN, Imsi, "10.99.0.11")
		if code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", code)
		}
	})

	t.Run("address outside pool CIDR is 400", func(t *testing.T) {
		code, _, _ := createStaticIp(url, client, token, staticDN, Imsi, "10.88.0.1")
		if code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", code)
		}
	})

	t.Run("IPv6 not /64-aligned is 400", func(t *testing.T) {
		code, _, _ := createStaticIp(url, client, token, staticDN, Imsi, "2001:db8:aaaa:1::5")
		if code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", code)
		}
	})

	t.Run("IPv6 aligned happy path", func(t *testing.T) {
		code, resp, err := createStaticIp(url, client, token, staticDN, Imsi, "2001:db8:aaaa:1::")
		if err != nil || code != http.StatusCreated {
			t.Fatalf("expected 201, got %d (%s)", code, resp.Error)
		}
	})

	t.Run("subscriber holds both families on the same DN", func(t *testing.T) {
		code, resp, err := listStaticIps(url, client, token)
		if err != nil || code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%v)", code, err)
		}

		families := map[string]string{}

		for _, it := range resp.Result.Items {
			if it.IMSI == Imsi {
				families[it.IPVersion] = it.Address
			}
		}

		if families["ipv4"] != "10.99.0.10" || families["ipv6"] != "2001:db8:aaaa:1::" {
			t.Fatalf("expected subscriber to hold both v4 and v6 pins, got %+v", families)
		}
	})

	t.Run("subscriber not found is 404", func(t *testing.T) {
		code, _, _ := createStaticIp(url, client, token, staticDN, "000000000000000", "10.99.0.12")
		if code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", code)
		}
	})

	t.Run("data network not found is 404", func(t *testing.T) {
		code, _, _ := createStaticIp(url, client, token, "no-such-dn", Imsi, "10.99.0.12")
		if code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", code)
		}
	})

	t.Run("data network not bound to profile is 409", func(t *testing.T) {
		code, _, _ := createStaticIp(url, client, token, staticDN, orphanIMSI, "10.99.0.13")
		if code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", code)
		}
	})

	t.Run("repin (PUT) happy path", func(t *testing.T) {
		code, resp, err := updateStaticIp(url, client, token, staticDN, Imsi, "ipv4", "10.99.0.20")
		if err != nil || code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", code, resp.Error)
		}

		_, list, _ := listStaticIps(url, client, token)

		found := false

		for _, it := range list.Result.Items {
			if it.IPVersion == "ipv4" && it.Address == "10.99.0.20" {
				found = true
			}
		}

		if !found {
			t.Fatalf("repinned address 10.99.0.20 not found in list")
		}
	})

	t.Run("mutation of an active reservation is 409", func(t *testing.T) {
		ctx := context.Background()

		dn, err := env.DB.GetDataNetwork(ctx, staticDN)
		if err != nil {
			t.Fatalf("GetDataNetwork: %s", err)
		}

		lease, err := env.DB.GetStaticLease(ctx, dn.ID, "ipv4", Imsi)
		if err != nil {
			t.Fatalf("GetStaticLease: %s", err)
		}

		if err := env.DB.UpdateLeaseSession(ctx, lease.ID, 1); err != nil {
			t.Fatalf("UpdateLeaseSession: %s", err)
		}

		if code, _, _ := updateStaticIp(url, client, token, staticDN, Imsi, "ipv4", "10.99.0.21"); code != http.StatusConflict {
			t.Fatalf("PUT active: expected 409, got %d", code)
		}

		if code, _, _ := deleteStaticIp(url, client, token, staticDN, Imsi, "ipv4"); code != http.StatusConflict {
			t.Fatalf("DELETE active: expected 409, got %d", code)
		}

		if err := env.DB.ClearStaticLeaseSession(ctx, lease.ID); err != nil {
			t.Fatalf("ClearStaticLeaseSession: %s", err)
		}
	})

	t.Run("delete happy path then 404", func(t *testing.T) {
		if code, resp, err := deleteStaticIp(url, client, token, staticDN, Imsi, "ipv4"); err != nil || code != http.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", code, resp.Error)
		}

		if code, _, _ := deleteStaticIp(url, client, token, staticDN, Imsi, "ipv4"); code != http.StatusNotFound {
			t.Fatalf("delete again: expected 404, got %d", code)
		}
	})

	t.Run("role gating", func(t *testing.T) {
		roClient := newTestClient(env.Server)

		roToken, err := createUserAndLogin(url, token, "static-ro@ellanetworks.com", RoleReadOnly, roClient)
		if err != nil {
			t.Fatalf("create read-only user: %s", err)
		}

		if code, _, _ := listStaticIps(url, roClient, roToken); code != http.StatusOK {
			t.Fatalf("read-only list: expected 200, got %d", code)
		}

		if code, _, _ := createStaticIp(url, roClient, roToken, staticDN, Imsi, "10.99.0.30"); code != http.StatusForbidden {
			t.Fatalf("read-only create: expected 403, got %d", code)
		}
	})
}

func errStr(r *CreateSliceResponse) string {
	if r == nil {
		return ""
	}

	return r.Error
}

func errStrDN(r *CreateDataNetworkResponse) string {
	if r == nil {
		return ""
	}

	return r.Error
}

func errStrProf(r *CreateProfileResponse) string {
	if r == nil {
		return ""
	}

	return r.Error
}

func errStrPol(r *CreatePolicyResponse) string {
	if r == nil {
		return ""
	}

	return r.Error
}

func errStrSub(r *CreateSubscriberResponse) string {
	if r == nil {
		return ""
	}

	return r.Error
}
