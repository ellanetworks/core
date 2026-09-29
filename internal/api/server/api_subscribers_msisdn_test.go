// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"path/filepath"
	"testing"
)

func TestSubscriberMSISDN(t *testing.T) {
	env, err := setupServer(filepath.Join(t.TempDir(), "db.sqlite3"))
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

	sc, _, err := createDataNetwork(url, client, token, &CreateDataNetworkParams{Name: "msisdn-dn", IPv4Pool: IPv4Pool, DNS: DNS, MTU: MTU})
	mustCreate("createDataNetwork", sc, err)

	sc, _, err = createSlice(url, client, token, &CreateSliceParams{Name: "msisdn-slice", Sst: 1})
	mustCreate("createSlice", sc, err)

	sc, _, err = createProfile(url, client, token, &CreateProfileParams{Name: TestProfileName, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps"})
	mustCreate("createProfile", sc, err)

	sc, _, err = createPolicy(url, client, token, &CreatePolicyParams{
		Name: "msisdn-policy", ProfileName: TestProfileName, SliceName: "msisdn-slice",
		SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", Var5qi: 9, Arp: 1, DataNetworkName: "msisdn-dn",
	})
	mustCreate("createPolicy", sc, err)

	const (
		firstIMSI  = "001010100007487"
		secondIMSI = "001010100007488"
	)

	sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{
		Imsi: firstIMSI, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber,
		ProfileName: TestProfileName, Msisdn: "+15551230001",
	})
	mustCreate("createSubscriber "+firstIMSI, sc, err)

	sc, _, err = createSubscriber(url, client, token, &CreateSubscriberParams{
		Imsi: secondIMSI, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: TestProfileName,
	})
	mustCreate("createSubscriber "+secondIMSI, sc, err)

	get := func(t *testing.T, imsi string) *SubscriberDetail {
		t.Helper()

		code, resp, err := getSubscriber(url, client, token, imsi)
		if err != nil {
			t.Fatalf("get %s: %s", imsi, err)
		}

		if code != http.StatusOK {
			t.Fatalf("get %s: expected 200, got %d (%q)", imsi, code, resp.Error)
		}

		return &resp.Result
	}

	t.Run("create stores the msisdn in E.164 form", func(t *testing.T) {
		if got := get(t, firstIMSI).Msisdn; got != "+15551230001" {
			t.Fatalf("msisdn = %q, want %q", got, "+15551230001")
		}

		if got := get(t, secondIMSI).Msisdn; got != "" {
			t.Fatalf("msisdn = %q, want unset", got)
		}
	})

	t.Run("list reports and searches the msisdn", func(t *testing.T) {
		code, resp, err := listSubscribersWithQuery(url, client, token, "search=%2B1555123")
		if err != nil {
			t.Fatalf("list: %s", err)
		}

		if code != http.StatusOK {
			t.Fatalf("list: expected 200, got %d (%q)", code, resp.Error)
		}

		if resp.Result.TotalCount != 1 || len(resp.Result.Items) != 1 || resp.Result.Items[0].Imsi != firstIMSI || resp.Result.Items[0].Msisdn != "+15551230001" {
			t.Fatalf("expected only %s, got total=%d items=%+v", firstIMSI, resp.Result.TotalCount, resp.Result.Items)
		}
	})

	t.Run("create rejects an msisdn already in use", func(t *testing.T) {
		code, resp, err := createSubscriber(url, client, token, &CreateSubscriberParams{
			Imsi: "001010100007489", Key: Key, Opc: Opc, SequenceNumber: SequenceNumber,
			ProfileName: TestProfileName, Msisdn: "+15551230001",
		})
		if err != nil {
			t.Fatalf("create: %s", err)
		}

		if code != http.StatusConflict {
			t.Fatalf("create: expected 409, got %d (%q)", code, resp.Error)
		}
	})

	t.Run("update rejects an msisdn already in use", func(t *testing.T) {
		code, resp, err := updateSubscriber(url, client, token, secondIMSI, &UpdateSubscriberParams{
			ProfileName: TestProfileName, Msisdn: "+15551230001",
		})
		if err != nil {
			t.Fatalf("update: %s", err)
		}

		if code != http.StatusConflict {
			t.Fatalf("update: expected 409, got %d (%q)", code, resp.Error)
		}
	})

	t.Run("invalid msisdns are rejected", func(t *testing.T) {
		for _, msisdn := range []string{"15551230001", "+0123", "+1234567890123456", "+1555-123", "++1555"} {
			code, resp, err := updateSubscriber(url, client, token, secondIMSI, &UpdateSubscriberParams{
				ProfileName: TestProfileName, Msisdn: msisdn,
			})
			if err != nil {
				t.Fatalf("update %q: %s", msisdn, err)
			}

			if code != http.StatusBadRequest {
				t.Fatalf("update %q: expected 400, got %d (%q)", msisdn, code, resp.Error)
			}
		}
	})

	t.Run("update changes then clears the msisdn", func(t *testing.T) {
		code, resp, err := updateSubscriber(url, client, token, firstIMSI, &UpdateSubscriberParams{
			ProfileName: TestProfileName, Msisdn: "+15551230002",
		})
		if err != nil || code != http.StatusOK {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		if got := get(t, firstIMSI).Msisdn; got != "+15551230002" {
			t.Fatalf("msisdn = %q, want %q", got, "+15551230002")
		}

		code, resp, err = updateSubscriber(url, client, token, firstIMSI, &UpdateSubscriberParams{ProfileName: TestProfileName})
		if err != nil || code != http.StatusOK {
			t.Fatalf("update: code=%d err=%v (%q)", code, err, resp.Error)
		}

		if got := get(t, firstIMSI).Msisdn; got != "" {
			t.Fatalf("msisdn = %q, want cleared", got)
		}
	})
}
