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

const subscribersPath = "/api/v1/subscribers"

type subscribersListEnvelope struct {
	Result server.ListSubscribersResponse `json:"result"`
	Error  string                         `json:"error,omitempty"`
}

type subscriberDetailEnvelope struct {
	Result server.SubscriberDetail `json:"result"`
	Error  string                  `json:"error,omitempty"`
}

func deregisterPath(imsi string) string {
	return subscribersPath + "/" + imsi + "/deregister"
}

func createOwnSubscriber(t *testing.T, env testEnv, client *http.Client, token string) {
	t.Helper()

	code, resp, err := createSubscriber(env.Server.URL, client, token, &CreateSubscriberParams{Imsi: Imsi, Key: Key, Opc: Opc, SequenceNumber: SequenceNumber, ProfileName: "default"})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create home subscriber: code=%d err=%v (%q)", code, err, resp.Error)
	}
}

func imsisOf(items []server.Subscriber) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		out = append(out, s.Imsi)
	}

	return out
}

func TestListSubscribersFiltersRoamers(t *testing.T) {
	env, client, token, partner := roamingPartnerWithRoamers(t, []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-1"},
		{imsi: "208940000000002", regType: db.UERegistrationTypeMME, node: "node-2"},
	})
	createOwnSubscriber(t, env, client, token)

	url := env.Server.URL + subscribersPath

	cases := map[string]struct {
		imsis   []string
		roamers int
	}{
		"":                                {imsis: []string{Imsi, "208930000000001", "208940000000002"}, roamers: 2},
		"?roaming=true":                   {imsis: []string{"208930000000001", "208940000000002"}, roamers: 2},
		"?roaming=false":                  {imsis: []string{Imsi}},
		"?roaming=true&search=2089400":    {imsis: []string{"208940000000002"}, roamers: 1},
		"?roaming=true&per_page=1&page=2": {imsis: []string{"208940000000002"}, roamers: 1},
	}

	for query, want := range cases {
		code, list, err := apiDo[subscribersListEnvelope](client, "GET", url+query, token, nil)
		if err != nil || code != http.StatusOK {
			t.Fatalf("GET subscribers%s: code=%d err=%v (%q)", query, code, err, list.Error)
		}

		got := imsisOf(list.Result.Items)
		if len(got) != len(want.imsis) {
			t.Fatalf("GET subscribers%s = %v, want %v", query, got, want.imsis)
		}

		roamers := 0

		for i, s := range list.Result.Items {
			if s.Imsi != want.imsis[i] {
				t.Fatalf("GET subscribers%s = %v, want %v", query, got, want.imsis)
			}

			if s.RoamingPartner != "" {
				roamers++

				if s.RoamingPartner != partner.Name || s.ProfileName != "default" {
					t.Fatalf("roamer = %+v", s)
				}
			}
		}

		if roamers != want.roamers {
			t.Fatalf("GET subscribers%s: %d roamers, want %d", query, roamers, want.roamers)
		}
	}

	code, paged, err := apiDo[subscribersListEnvelope](client, "GET", url+"?roaming=true&per_page=1", token, nil)
	if err != nil || code != http.StatusOK || paged.Result.TotalCount != 2 {
		t.Fatalf("roamers total count: code=%d err=%v resp=%+v", code, err, paged)
	}

	code, all, err := apiDo[subscribersListEnvelope](client, "GET", url, token, nil)
	if err != nil || code != http.StatusOK || all.Result.TotalCount != 3 {
		t.Fatalf("total count: code=%d err=%v resp=%+v", code, err, all)
	}

	for _, query := range []string{"?roaming=yes", "?roaming=1", "?roaming=TRUE"} {
		if code, resp, _ := apiDo[subscribersListEnvelope](client, "GET", url+query, token, nil); code != http.StatusBadRequest || resp.Error != "roaming must be true or false" {
			t.Fatalf("GET subscribers%s: code=%d (%q), want 400", query, code, resp.Error)
		}
	}
}

func TestRoamerDetailAndOwnSubscriberOnlyOperations(t *testing.T) {
	const roamer = "208930000000001"

	env, client, token, partner := roamingPartnerWithRoamers(t, []heldRoamer{{imsi: roamer, regType: db.UERegistrationTypeMME, node: "node-1"}})
	createOwnSubscriber(t, env, client, token)

	url := env.Server.URL + subscribersPath

	code, detail, err := apiDo[subscriberDetailEnvelope](client, "GET", url+"/"+roamer, token, nil)
	if err != nil || code != http.StatusOK {
		t.Fatalf("GET roamer: code=%d err=%v (%q)", code, err, detail.Error)
	}

	if detail.Result.Imsi != roamer || detail.Result.RoamingPartner != partner.Name || detail.Result.ProfileName != "default" || detail.Result.IMS != nil {
		t.Fatalf("roamer detail = %+v", detail.Result)
	}

	code, own, err := apiDo[subscriberDetailEnvelope](client, "GET", url+"/"+Imsi, token, nil)
	if err != nil || code != http.StatusOK || own.Result.RoamingPartner != "" {
		t.Fatalf("GET home subscriber: code=%d err=%v resp=%+v", code, err, own)
	}

	for _, method := range []string{"PUT", "DELETE"} {
		code, resp, err := apiDo[roamingMessageEnvelope](client, method, url+"/"+roamer, token, map[string]string{"profile_name": "default"})
		if err != nil || code != http.StatusConflict || resp.Error != "Roamers cannot be updated or deleted; deregister them instead" {
			t.Fatalf("%s roamer: code=%d err=%v (%q), want 409", method, code, err, resp.Error)
		}
	}

	if _, err := env.DB.GetSubscriberOrVisitor(context.Background(), roamer); err != nil {
		t.Fatalf("roamer after rejected PUT/DELETE: %v", err)
	}

	if code, _, err := apiDo[map[string]any](client, "GET", url+"/"+roamer+"/credentials", token, nil); err != nil || code != http.StatusNotFound {
		t.Fatalf("GET roamer credentials: code=%d err=%v, want 404", code, err)
	}
}

func TestDeregisterSubscriberAnswers(t *testing.T) {
	env, client, token, _ := roamingPartnerWithRoamers(t, nil)
	createOwnSubscriber(t, env, client, token)

	cases := []struct {
		imsi    string
		code    int
		message string
	}{
		{"abc", http.StatusBadRequest, "Invalid IMSI format"},
		{"208930000000009", http.StatusNotFound, "Subscriber not found"},
		{Imsi, http.StatusConflict, "Only roamers can be deregistered"},
	}

	for _, c := range cases {
		code, resp, err := apiDo[roamingMessageEnvelope](client, "POST", env.Server.URL+deregisterPath(c.imsi), token, nil)
		if err != nil || code != c.code || resp.Error != c.message {
			t.Fatalf("deregister %s: code=%d err=%v (%q), want %d (%q)", c.imsi, code, err, resp.Error, c.code, c.message)
		}
	}

	readOnly := newTestClient(env.Server)

	readOnlyToken, err := createUserAndLogin(env.Server.URL, token, "deregister-ro@ellanetworks.com", RoleReadOnly, readOnly)
	if err != nil {
		t.Fatalf("create read-only user: %s", err)
	}

	if code, _, err := apiDo[roamingMessageEnvelope](readOnly, "POST", env.Server.URL+deregisterPath(Imsi), readOnlyToken, nil); err != nil || code != http.StatusForbidden {
		t.Fatalf("read-only deregister: code=%d err=%v, want 403", code, err)
	}
}

func TestDeregisteringARoamerWithdrawsOnlyThatRoamer(t *testing.T) {
	held := []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"},
		{imsi: "208930000000002", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-2"},
	}

	env, client, token, partner := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 10*time.Second)

	nodes := startFakeNodes(t, env, partner.ID, held)

	code, resp, err := apiDo[roamingMessageEnvelope](client, "POST", env.Server.URL+deregisterPath("208930000000002"), token, nil)
	if err != nil || code != http.StatusOK || resp.Result.Message != "Roamer deregistered successfully" {
		t.Fatalf("deregister roamer: code=%d err=%v (%q)", code, err, resp.Error)
	}

	if withdrawn, _ := nodes.deregistered(); len(withdrawn) != 1 || withdrawn[0] != "208930000000002" {
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

	env, client, token, partner := roamingPartnerWithRoamers(t, []heldRoamer{{imsi: imsi, regType: db.UERegistrationTypeMME, node: "node-down"}})
	server.SetRoamerWithdrawalTimeoutForTest(t, 50*time.Millisecond)

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "POST", env.Server.URL+deregisterPath(imsi), token, nil); err != nil || code != http.StatusConflict || resp.Error == "" {
		t.Fatalf("deregister a roamer nobody releases: code=%d err=%v (%q), want 409", code, err, resp.Error)
	}

	ctx := context.Background()

	if _, err := env.DB.RoamingWithdrawalOf(ctx, imsi, db.UERegistrationTypeMME); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("withdrawal after the timeout: %v, want cleared", err)
	}

	if got, err := env.DB.GetRoamingPartner(ctx, partner.ID); err != nil || len(got.Withdrawals) != 0 {
		t.Fatalf("partner withdrawals = %+v, %v, want none", got, err)
	}
}
