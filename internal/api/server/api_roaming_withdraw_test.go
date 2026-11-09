// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/db"
)

type heldRoamer struct {
	imsi    string
	regType string
	node    string
}

type fakeNodes struct {
	database *db.Database
	held     []heldRoamer

	mu        sync.Mutex
	withdrawn []string
}

func (n *fakeNodes) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Millisecond):
		}

		for _, r := range n.held {
			v, err := n.database.GetVisitor(ctx, r.imsi)
			if err != nil || (!v.Withdrawing(time.Now()) && !v.Deregistering(time.Now())) {
				continue
			}

			if err := n.database.PurgeUERegistration(ctx, r.imsi, r.regType, r.node); err != nil {
				continue
			}

			n.mu.Lock()
			if !slices.Contains(n.withdrawn, r.imsi) {
				n.withdrawn = append(n.withdrawn, r.imsi)
			}
			n.mu.Unlock()
		}
	}
}

func (n *fakeNodes) deregistered() []string {
	n.mu.Lock()
	defer n.mu.Unlock()

	return append([]string(nil), n.withdrawn...)
}

func roamingPartnerWithRoamers(t *testing.T, held []heldRoamer) (testEnv, *http.Client, string, roamingPartnerBody) {
	t.Helper()

	env, client, token := newAuthedTestEnv(t)

	return env, client, token, createPartnerWithRoamers(t, env, client, token, held)
}

func createPartnerWithRoamers(t *testing.T, env testEnv, client *http.Client, token string, held []heldRoamer) roamingPartnerBody {
	t.Helper()

	url := env.Server.URL

	code, peer, err := apiDo[roamingDiameterPeerEnvelope](client, "POST", url+roamingDiameterPeersPath, token, roamingDiameterPeerBody{OriginHost: "dea.epc.mnc093.mcc208.3gppnetwork.org", Address: "192.0.2.10"})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create Diameter peer: code=%d err=%v (%q)", code, err, peer.Error)
	}

	body := partner5G(t, "partner")
	body.PLMNs = plmnList("208", "93", "208", "94")
	body.DiameterPeerID = peer.Result.ID

	code, created, err := apiDo[roamingPartnerEnvelope](client, "POST", url+roamingPartnersPath, token, body)
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create partner: code=%d err=%v (%q)", code, err, created.Error)
	}

	body.ID = created.Result.ID

	for _, r := range held {
		if _, err := env.DB.RegisterVisitorUE(context.Background(), r.imsi, body.ID, r.regType, r.node, ""); err != nil {
			t.Fatalf("RegisterVisitorUE %s: %s", r.imsi, err)
		}
	}

	return body
}

func startFakeNodes(t *testing.T, env testEnv, held []heldRoamer) *fakeNodes {
	t.Helper()

	nodes := &fakeNodes{database: env.DB, held: held}
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	wg.Go(func() { nodes.run(ctx) })

	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	return nodes
}

func TestDeletingARoamingPartnerDeregistersItsRoamersOnEveryNode(t *testing.T) {
	held := []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"},
		{imsi: "208940000000002", regType: db.UERegistrationTypeMME, node: "node-2"},
		{imsi: "208940000000003", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-3"},
	}

	env, client, token, partner := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 10*time.Second)

	nodes := startFakeNodes(t, env, held)

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamingPartnersPath+"/"+partner.ID, token, nil); err != nil || code != http.StatusOK {
		t.Fatalf("delete a partner with roamers: code=%d err=%v (%q), want 200", code, err, resp.Error)
	}

	if withdrawn := nodes.deregistered(); len(withdrawn) != len(held) {
		t.Fatalf("deregistered = %v, want every roamer of the partner", withdrawn)
	}

	if _, err := env.DB.GetRoamingPartner(context.Background(), partner.ID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("partner after delete: %v, want not found", err)
	}

	for _, r := range held {
		if _, err := env.DB.GetSubscriberOrVisitor(context.Background(), r.imsi); !errors.Is(err, db.ErrNotFound) {
			t.Fatalf("visitor %s after delete: %v, want not found", r.imsi, err)
		}
	}
}

func TestDeletingARoamingPartnerWithAnUnreachableRoamerAnswersConflict(t *testing.T) {
	held := []heldRoamer{{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"}}

	env, client, token, partner := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 50*time.Millisecond)

	if code, _, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamingPartnersPath+"/"+partner.ID, token, nil); err != nil || code != http.StatusConflict {
		t.Fatalf("delete a partner with an unreachable roamer: code=%d err=%v, want 409", code, err)
	}

	got, err := env.DB.GetRoamingPartner(context.Background(), partner.ID)
	if err != nil || got.Withdrawing(time.Now()) {
		t.Fatalf("partner after a failed delete = %+v, %v, want it kept and no longer withdrawing", got, err)
	}

	if _, err := env.DB.RegisterVisitorUE(context.Background(), "208930000000002", partner.ID, db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("register a roamer after a failed delete: %v", err)
	}
}

func TestRemovingAnAgreementWithAttachedRoamersWithdrawsThem(t *testing.T) {
	held := []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"},
		{imsi: "208940000000002", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-1"},
	}

	for name, mutate := range map[string]func(*roamingPartnerBody){
		"PLMN":         func(b *roamingPartnerBody) { b.PLMNs = plmnList("208", "93") },
		"4G agreement": func(b *roamingPartnerBody) { b.DiameterPeerID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			env, client, token, partner := roamingPartnerWithRoamers(t, held)
			server.SetRoamerWithdrawalTimeoutForTest(t, 10*time.Second)

			nodes := startFakeNodes(t, env, held)

			body := partner
			mutate(&body)

			if code, resp, err := apiDo[roamingMessageEnvelope](client, "PUT", env.Server.URL+roamingPartnersPath+"/"+partner.ID, token, body); err != nil || code != http.StatusOK {
				t.Fatalf("remove the %s with attached roamers: code=%d err=%v (%q), want 200", name, code, err, resp.Error)
			}

			if withdrawn := nodes.deregistered(); len(withdrawn) != len(held) {
				t.Fatalf("deregistered = %v, want every roamer of the partner", withdrawn)
			}

			got, err := env.DB.GetRoamingPartner(context.Background(), partner.ID)
			if err != nil || got.Withdrawing(time.Now()) || (len(got.PLMNs) != 2) == (got.DiameterPeerID == nil) {
				t.Fatalf("partner after the update = %+v, %v, want the %s removed and no longer withdrawing", got, err, name)
			}
		})
	}
}

func TestRemovingAnAgreementWithAnUnreachableRoamerAnswersConflict(t *testing.T) {
	held := []heldRoamer{{imsi: "208940000000001", regType: db.UERegistrationTypeMME, node: "node-1"}}

	env, client, token, partner := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 50*time.Millisecond)

	body := partner
	body.PLMNs = plmnList("208", "93")

	if code, _, err := apiDo[roamingMessageEnvelope](client, "PUT", env.Server.URL+roamingPartnersPath+"/"+partner.ID, token, body); err != nil || code != http.StatusConflict {
		t.Fatalf("remove a PLMN with an unreachable roamer: code=%d err=%v, want 409", code, err)
	}

	got, err := env.DB.GetRoamingPartner(context.Background(), partner.ID)
	if err != nil || len(got.PLMNs) != 2 || got.Withdrawing(time.Now()) {
		t.Fatalf("partner after a failed update = %+v, %v, want it unchanged and no longer withdrawing", got, err)
	}
}

func TestPartnerChangeWithoutAffectedRoamersSkipsTheWithdrawal(t *testing.T) {
	held := []heldRoamer{{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"}}

	env, client, token, partner := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 50*time.Millisecond)

	partner.PLMNs = plmnList("208", "93")

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "PUT", env.Server.URL+roamingPartnersPath+"/"+partner.ID, token, partner); err != nil || code != http.StatusOK {
		t.Fatalf("remove a PLMN without roamers: code=%d err=%v (%q), want 200", code, err, resp.Error)
	}

	if _, err := env.DB.GetSubscriberOrVisitor(context.Background(), "208930000000001"); err != nil {
		t.Fatalf("a roamer outside the change was removed: %v", err)
	}
}
