// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/db"
)

const (
	subscribersPath = "/api/v1/subscribers"
	roamersPath     = "/api/v1/roamers"
)

type subscribersListEnvelope struct {
	Result server.ListSubscribersResponse `json:"result"`
	Error  string                         `json:"error,omitempty"`
}

type roamersListEnvelope struct {
	Result server.ListRoamersResponse `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type roamerDetailEnvelope struct {
	Result server.RoamerDetail `json:"result"`
	Error  string              `json:"error,omitempty"`
}

func roamerPath(imsi string) string {
	return roamersPath + "/" + imsi
}

func createOwnSubscriber(t *testing.T, env testEnv, client *http.Client, token string) {
	t.Helper()

	code, resp, err := createSubscriber(env.Server.URL, client, token, &CreateSubscriberParams{Imsi: Imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: "default"})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create home subscriber: code=%d err=%v (%q)", code, err, resp.Error)
	}
}

func roamerIMSIs(items []server.Roamer) []string {
	out := make([]string, 0, len(items))
	for _, r := range items {
		out = append(out, r.Imsi)
	}

	return out
}

func TestSubscribersListOnlyHomeSubscribers(t *testing.T) {
	env, client, token, _ := roamingPartnerWithRoamers(t, []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-1"},
	})
	createOwnSubscriber(t, env, client, token)

	code, list, err := apiDo[subscribersListEnvelope](client, "GET", env.Server.URL+subscribersPath+"?search=2089", token, nil)
	if err != nil || code != http.StatusOK || list.Result.TotalCount != 0 {
		t.Fatalf("GET subscribers?search=2089: code=%d err=%v resp=%+v", code, err, list)
	}

	code, list, err = apiDo[subscribersListEnvelope](client, "GET", env.Server.URL+subscribersPath, token, nil)
	if err != nil || code != http.StatusOK || list.Result.TotalCount != 1 || len(list.Result.Items) != 1 || list.Result.Items[0].Imsi != Imsi {
		t.Fatalf("GET subscribers: code=%d err=%v resp=%+v", code, err, list)
	}
}

func TestListRoamers(t *testing.T) {
	env, client, token, partner := roamingPartnerWithRoamers(t, []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-1"},
		{imsi: "208940000000002", regType: db.UERegistrationTypeMME, node: "node-2"},
	})
	createOwnSubscriber(t, env, client, token)

	if err := env.DB.SetVisitorMSISDN(context.Background(), "208940000000002", "33612345678"); err != nil {
		t.Fatalf("SetVisitorMSISDN: %s", err)
	}

	connectAPIRadio(env.AMF, "gnb-1")

	if code, resp, err := createDataNetwork(env.Server.URL, client, token, &CreateDataNetworkParams{Name: "other-dn", IPv4Pool: "10.99.0.0/24", DNS: "8.8.8.8", MTU: 1400}); err != nil || code != http.StatusCreated {
		t.Fatalf("create data network: code=%d err=%v (%+v)", code, err, resp)
	}

	url := env.Server.URL + roamersPath

	cases := map[string]struct {
		imsis []string
		total int
	}{
		"":                             {imsis: []string{"208930000000001", "208940000000002"}, total: 2},
		"?search=2089400":              {imsis: []string{"208940000000002"}, total: 1},
		"?search=%2B336":               {imsis: []string{"208940000000002"}, total: 1},
		"?search=" + Imsi:              {imsis: []string{}, total: 0},
		"?per_page=1&page=2":           {imsis: []string{"208940000000002"}, total: 2},
		"?page=3&per_page=1":           {imsis: []string{}, total: 2},
		"?partner=" + partner.Name:     {imsis: []string{"208930000000001", "208940000000002"}, total: 2},
		"?partner=partner&search=2089": {imsis: []string{"208930000000001", "208940000000002"}, total: 2},
		"?data_network=internet":       {imsis: []string{"208930000000001", "208940000000002"}, total: 2},
		"?data_network=other-dn":       {imsis: []string{}, total: 0},
		"?radio=gnb-1":                 {imsis: []string{}, total: 0},
	}

	for query, want := range cases {
		code, list, err := apiDo[roamersListEnvelope](client, "GET", url+query, token, nil)
		if err != nil || code != http.StatusOK {
			t.Fatalf("GET roamers%s: code=%d err=%v (%q)", query, code, err, list.Error)
		}

		got := roamerIMSIs(list.Result.Items)
		if len(got) != len(want.imsis) || list.Result.TotalCount != want.total {
			t.Fatalf("GET roamers%s = %v (total %d), want %v (total %d)", query, got, list.Result.TotalCount, want.imsis, want.total)
		}

		for i, imsi := range want.imsis {
			if got[i] != imsi {
				t.Fatalf("GET roamers%s = %v, want %v", query, got, want.imsis)
			}
		}
	}

	code, list, err := apiDo[roamersListEnvelope](client, "GET", url, token, nil)
	if err != nil || code != http.StatusOK {
		t.Fatalf("GET roamers: code=%d err=%v", code, err)
	}

	first, second := list.Result.Items[0], list.Result.Items[1]

	if first.Partner != partner.Name || first.HomePLMN == nil || first.HomePLMN.Mcc != "208" || first.HomePLMN.Mnc != "93" || first.Msisdn != "" || first.Since == "" {
		t.Fatalf("first roamer = %+v", first)
	}

	if second.HomePLMN == nil || second.HomePLMN.Mnc != "94" || second.Msisdn != "+33612345678" {
		t.Fatalf("second roamer = %+v", second)
	}

	if _, err := time.Parse(time.RFC3339, first.Since); err != nil {
		t.Fatalf("since = %q: %v", first.Since, err)
	}

	errorCases := map[string]struct {
		code    int
		message string
	}{
		"?partner=unknown":      {http.StatusNotFound, "Roaming partner not found"},
		"?radio=unknown":        {http.StatusNotFound, "Radio not found"},
		"?data_network=unknown": {http.StatusNotFound, "Data Network not found"},
		"?page=0":               {http.StatusBadRequest, "page must be >= 1"},
		"?per_page=101":         {http.StatusBadRequest, "per_page must be between 1 and 100"},
	}

	for query, want := range errorCases {
		if code, resp, _ := apiDo[roamersListEnvelope](client, "GET", url+query, token, nil); code != want.code || resp.Error != want.message {
			t.Fatalf("GET roamers%s: code=%d (%q), want %d (%q)", query, code, resp.Error, want.code, want.message)
		}
	}
}

func TestRoamerDetail(t *testing.T) {
	const roamer = "208930000000001"

	env, client, token, partner := roamingPartnerWithRoamers(t, []heldRoamer{{imsi: roamer, regType: db.UERegistrationTypeMME, node: "node-1"}})
	createOwnSubscriber(t, env, client, token)

	code, detail, err := apiDo[roamerDetailEnvelope](client, "GET", env.Server.URL+roamerPath(roamer), token, nil)
	if err != nil || code != http.StatusOK {
		t.Fatalf("GET roamer: code=%d err=%v (%q)", code, err, detail.Error)
	}

	got := detail.Result
	if got.Imsi != roamer || got.Partner != partner.Name || got.HomePLMN == nil || got.HomePLMN.Mcc != "208" || got.HomePLMN.Mnc != "93" || got.Since == "" || got.Registrations == nil || got.Sessions == nil {
		t.Fatalf("roamer detail = %+v", got)
	}

	cases := []struct {
		path    string
		code    int
		message string
	}{
		{roamerPath("abc"), http.StatusBadRequest, "Invalid IMSI format"},
		{roamerPath("208930000000009"), http.StatusNotFound, "Roamer not found"},
		{roamerPath(Imsi), http.StatusNotFound, "Roamer not found"},
		{subscribersPath + "/" + roamer, http.StatusNotFound, "Subscriber not found"},
	}

	for _, c := range cases {
		if code, resp, _ := apiDo[roamerDetailEnvelope](client, "GET", env.Server.URL+c.path, token, nil); code != c.code || resp.Error != c.message {
			t.Fatalf("GET %s: code=%d (%q), want %d (%q)", c.path, code, resp.Error, c.code, c.message)
		}
	}
}

func TestSubscriberEndpointsIgnoreRoamers(t *testing.T) {
	const roamer = "208930000000001"

	env, client, token, _ := roamingPartnerWithRoamers(t, []heldRoamer{{imsi: roamer, regType: db.UERegistrationTypeMME, node: "node-1"}})

	url := env.Server.URL + subscribersPath + "/" + roamer

	for _, method := range []string{"PUT", "DELETE"} {
		if code, resp, err := apiDo[roamingMessageEnvelope](client, method, url, token, map[string]string{"profile_name": "default"}); err != nil || code != http.StatusNotFound {
			t.Fatalf("%s roamer as a subscriber: code=%d err=%v (%q), want 404", method, code, err, resp.Error)
		}
	}

	if code, _, err := apiDo[map[string]any](client, "GET", url+"/credentials", token, nil); err != nil || code != http.StatusNotFound {
		t.Fatalf("GET roamer credentials: code=%d err=%v, want 404", code, err)
	}

	if _, err := env.DB.GetVisitor(context.Background(), roamer); err != nil {
		t.Fatalf("roamer after subscriber PUT/DELETE: %v", err)
	}
}

func TestDeregisterRoamerAnswers(t *testing.T) {
	env, client, token, _ := roamingPartnerWithRoamers(t, nil)
	createOwnSubscriber(t, env, client, token)

	cases := []struct {
		imsi    string
		code    int
		message string
	}{
		{"abc", http.StatusBadRequest, "Invalid IMSI format"},
		{"208930000000009", http.StatusNotFound, "Roamer not found"},
		{Imsi, http.StatusNotFound, "Roamer not found"},
	}

	for _, c := range cases {
		code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamerPath(c.imsi), token, nil)
		if err != nil || code != c.code || resp.Error != c.message {
			t.Fatalf("deregister %s: code=%d err=%v (%q), want %d (%q)", c.imsi, code, err, resp.Error, c.code, c.message)
		}
	}

	if _, err := env.DB.GetSubscriber(context.Background(), Imsi); err != nil {
		t.Fatalf("home subscriber after DELETE roamers/%s: %v", Imsi, err)
	}

	readOnly := newTestClient(env.Server)

	readOnlyToken, err := createUserAndLogin(env.Server.URL, token, "deregister-ro@ellanetworks.com", RoleReadOnly, readOnly)
	if err != nil {
		t.Fatalf("create read-only user: %s", err)
	}

	if code, _, err := apiDo[roamingMessageEnvelope](readOnly, "DELETE", env.Server.URL+roamerPath("208930000000009"), readOnlyToken, nil); err != nil || code != http.StatusForbidden {
		t.Fatalf("read-only deregister: code=%d err=%v, want 403", code, err)
	}

	if code, _, err := apiDo[roamersListEnvelope](readOnly, "GET", env.Server.URL+roamersPath, readOnlyToken, nil); err != nil || code != http.StatusOK {
		t.Fatalf("read-only list roamers: code=%d err=%v, want 200", code, err)
	}
}

func TestDeregisteringARoamerWithdrawsOnlyThatRoamer(t *testing.T) {
	held := []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"},
		{imsi: "208930000000002", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-2"},
	}

	env, client, token, partner := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 10*time.Second)

	nodes := startFakeNodes(t, env, held)

	code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamerPath("208930000000002"), token, nil)
	if err != nil || code != http.StatusOK || resp.Result.Message != "Roamer deregistered successfully" {
		t.Fatalf("deregister roamer: code=%d err=%v (%q)", code, err, resp.Error)
	}

	if withdrawn := nodes.deregistered(); len(withdrawn) != 1 || withdrawn[0] != "208930000000002" {
		t.Fatalf("deregistered = %v, want only the chosen roamer", withdrawn)
	}

	ctx := context.Background()

	if _, err := env.DB.GetSubscriberOrVisitor(ctx, "208930000000002"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor after deregistration: %v, want not found", err)
	}

	if _, err := env.DB.GetSubscriberOrVisitor(ctx, "208930000000001"); err != nil {
		t.Fatalf("the other roamer was removed: %v", err)
	}

	if _, err := env.DB.RegisterVisitorUE(ctx, "208930000000002", partner.ID, db.UERegistrationTypeAMF3GPPAccess, "node-2", ""); err != nil {
		t.Fatalf("the roamer cannot register again after its deregistration: %v", err)
	}
}

func TestDeregisteringAnUnreachableRoamerAnswersConflict(t *testing.T) {
	const imsi = "208930000000001"

	env, client, token, _ := roamingPartnerWithRoamers(t, []heldRoamer{{imsi: imsi, regType: db.UERegistrationTypeMME, node: "node-down"}})
	server.SetRoamerWithdrawalTimeoutForTest(t, 50*time.Millisecond)

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamerPath(imsi), token, nil); err != nil || code != http.StatusConflict || resp.Error == "" {
		t.Fatalf("deregister a roamer nobody releases: code=%d err=%v (%q), want 409", code, err, resp.Error)
	}

	ctx := context.Background()

	if v, err := env.DB.GetVisitor(ctx, imsi); err != nil || v.Deregistering(time.Now()) {
		t.Fatalf("roamer after the timeout = %+v, %v, want it kept and no longer deregistering", v, err)
	}
}

func TestDeregisteringARoamerOutlivesTheClient(t *testing.T) {
	const imsi = "208930000000001"

	held := []heldRoamer{{imsi: imsi, regType: db.UERegistrationTypeMME, node: "node-1"}}

	env, client, token, _ := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 10*time.Second)

	impatient := *client
	impatient.Timeout = 100 * time.Millisecond

	if _, _, err := apiDo[roamingMessageEnvelope](&impatient, "DELETE", env.Server.URL+roamerPath(imsi), token, nil); err == nil {
		t.Fatal("the request finished before any node released the roamer")
	}

	ctx := context.Background()

	if v, err := env.DB.GetVisitor(ctx, imsi); err != nil || !v.Deregistering(time.Now()) {
		t.Fatalf("roamer after the client gave up = %+v, %v, want it still deregistering", v, err)
	}

	startFakeNodes(t, env, held)

	deadline := time.Now().Add(5 * time.Second)

	for {
		_, err := env.DB.GetVisitor(ctx, imsi)
		if errors.Is(err, db.ErrNotFound) {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("roamer still present after its node released it: %v", err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}
