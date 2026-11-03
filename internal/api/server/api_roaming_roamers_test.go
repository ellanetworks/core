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
	"github.com/ellanetworks/core/internal/roaming"
)

const roamersPath = "/api/v1/roaming/roamers"

type roamersEnvelope struct {
	Result server.ListRoamersResponse `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

type roamerEnvelope struct {
	Result server.Roamer `json:"result"`
	Error  string        `json:"error,omitempty"`
}

func setLocalRoamers(t *testing.T, roamers []roaming.LiveRoamer) {
	t.Helper()

	server.SetLocalRoamers(func() []roaming.LiveRoamer { return roamers })
	t.Cleanup(func() { server.SetLocalRoamers(func() []roaming.LiveRoamer { return nil }) })
}

func TestListRoamers(t *testing.T) {
	env, client, token, partner := roamingPartnerWithRoamers(t, []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-1"},
		{imsi: "208940000000002", regType: db.UERegistrationTypeMME, node: "node-2"},
	})
	setLocalRoamers(t, nil)

	if err := env.DB.IncrementRoamingDailyUsageBatch(context.Background(), []db.RoamingDailyUsage{
		{EpochDay: db.DaysSinceEpoch(time.Now()), IMSI: "208930000000001", Mcc: "208", Mnc: "93", BytesUplink: 10, BytesDownlink: 20},
	}); err != nil {
		t.Fatalf("IncrementRoamingDailyUsageBatch: %s", err)
	}

	code, list, err := apiDo[roamersEnvelope](client, "GET", env.Server.URL+roamersPath, token, nil)
	if err != nil || code != http.StatusOK || len(list.Result.Items) != 2 {
		t.Fatalf("list roamers: code=%d err=%v resp=%+v", code, err, list)
	}

	first := list.Result.Items[0]
	if first.IMSI != "208930000000001" || first.PartnerID != partner.ID || first.PartnerName != partner.Name || first.HomePLMN.Mcc != "208" || first.HomePLMN.Mnc != "93" {
		t.Fatalf("roamer = %+v", first)
	}

	if len(first.Registrations) != 1 || first.Registrations[0].RAT != "5G" || first.Registrations[0].NodeID != "node-1" || first.Registrations[0].Reachable {
		t.Fatalf("registrations on a node outside the cluster = %+v", first.Registrations)
	}

	if first.UsageToday == nil || first.UsageToday.TotalBytes != 30 {
		t.Fatalf("usage today = %+v, want 30 bytes", first.UsageToday)
	}

	if second := list.Result.Items[1]; second.HomePLMN.Mnc != "94" || second.Registrations[0].RAT != "4G" || second.UsageToday == nil || second.UsageToday.TotalBytes != 0 {
		t.Fatalf("second roamer = %+v", second)
	}

	code, one, err := apiDo[roamerEnvelope](client, "GET", env.Server.URL+roamersPath+"/208930000000001", token, nil)
	if err != nil || code != http.StatusOK || one.Result.IMSI != "208930000000001" {
		t.Fatalf("get roamer: code=%d err=%v resp=%+v", code, err, one)
	}

	for path, want := range map[string]int{"/208930000000009": http.StatusNotFound, "/abc": http.StatusBadRequest} {
		if code, _, err := apiDo[roamerEnvelope](client, "GET", env.Server.URL+roamersPath+path, token, nil); err != nil || code != want {
			t.Fatalf("GET %s: code=%d err=%v, want %d", path, code, err, want)
		}
	}
}

func TestRoamerUsageNeedsSubscriberUsagePermission(t *testing.T) {
	env, _, adminToken, partner := roamingPartnerWithRoamers(t, []heldRoamer{{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"}})
	setLocalRoamers(t, nil)

	manager := newTestClient(env.Server)

	managerToken, err := createUserAndLogin(env.Server.URL, adminToken, "roamers-nm@ellanetworks.com", RoleNetworkManager, manager)
	if err != nil {
		t.Fatalf("create network manager: %s", err)
	}

	code, list, err := apiDo[roamersEnvelope](manager, "GET", env.Server.URL+roamersPath, managerToken, nil)
	if err != nil || code != http.StatusOK || len(list.Result.Items) != 1 || list.Result.Items[0].PartnerID != partner.ID {
		t.Fatalf("network manager list roamers: code=%d err=%v resp=%+v", code, err, list)
	}

	if list.Result.Items[0].UsageToday != nil {
		t.Fatalf("usage today shown without %s", server.PermGetSubscriberUsage)
	}

	readOnly := newTestClient(env.Server)

	readOnlyToken, err := createUserAndLogin(env.Server.URL, adminToken, "roamers-ro@ellanetworks.com", RoleReadOnly, readOnly)
	if err != nil {
		t.Fatalf("create read-only user: %s", err)
	}

	if code, _, err := apiDo[roamingMessageEnvelope](readOnly, "DELETE", env.Server.URL+roamersPath+"/208930000000001", readOnlyToken, nil); err != nil || code != http.StatusForbidden {
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

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamersPath+"/208930000000002", token, nil); err != nil || code != http.StatusOK {
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

	if code, _, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamersPath+"/208930000000009", token, nil); err != nil || code != http.StatusNotFound {
		t.Fatalf("deregister an unknown roamer: code=%d err=%v, want 404", code, err)
	}
}

func TestDeregisteringAnUnreachableRoamerAnswersConflict(t *testing.T) {
	const imsi = "208930000000001"

	env, client, token, partner := roamingPartnerWithRoamers(t, []heldRoamer{{imsi: imsi, regType: db.UERegistrationTypeMME, node: "node-down"}})
	server.SetRoamerWithdrawalTimeoutForTest(t, 50*time.Millisecond)

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamersPath+"/"+imsi, token, nil); err != nil || code != http.StatusConflict || resp.Error == "" {
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
