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

	mu         sync.Mutex
	withdrawn  []string
	withdrawal []db.RoamingWithdrawal
}

func (n *fakeNodes) run(ctx context.Context, partnerID string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Millisecond):
		}

		if p, err := n.database.GetRoamingPartner(ctx, partnerID); err == nil && len(p.Withdrawals) > 0 {
			n.mu.Lock()
			n.withdrawal = p.Withdrawals
			n.mu.Unlock()
		}

		for _, r := range n.held {
			if _, err := n.database.RoamingWithdrawalOf(ctx, r.imsi, r.regType); err != nil {
				continue
			}

			if err := n.database.PurgeUERegistration(ctx, r.imsi, r.regType, r.node); err != nil {
				continue
			}

			if _, err := n.database.DeleteVisitorIfUnregistered(ctx, r.imsi); err != nil {
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

func (n *fakeNodes) deregistered() ([]string, []db.RoamingWithdrawal) {
	n.mu.Lock()
	defer n.mu.Unlock()

	return append([]string(nil), n.withdrawn...), append([]db.RoamingWithdrawal(nil), n.withdrawal...)
}

func roamingPartnerWithRoamers(t *testing.T, held []heldRoamer) (testEnv, *http.Client, string, roamingPartnerBody) {
	t.Helper()

	env, client, token := newAuthedTestEnv(t)

	return env, client, token, createPartnerWithRoamers(t, env, client, token, held)
}

func createPartnerWithRoamers(t *testing.T, env testEnv, client *http.Client, token string, held []heldRoamer) roamingPartnerBody {
	t.Helper()

	url := env.Server.URL

	code, peer, err := apiDo[roamingDiameterPeerEnvelope](client, "POST", url+roamingDiameterPeersPath, token, roamingDiameterPeerBody{DiameterIdentity: "dea.epc.mnc093.mcc208.3gppnetwork.org", Address: "192.0.2.10"})
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

func startFakeNodes(t *testing.T, env testEnv, partnerID string, held []heldRoamer) *fakeNodes {
	t.Helper()

	nodes := &fakeNodes{database: env.DB, held: held}
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	wg.Go(func() { nodes.run(ctx, partnerID) })

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

	nodes := startFakeNodes(t, env, partner.ID, held)

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "DELETE", env.Server.URL+roamingPartnersPath+"/"+partner.ID, token, nil); err != nil || code != http.StatusOK {
		t.Fatalf("delete a partner with roamers: code=%d err=%v (%q), want 200", code, err, resp.Error)
	}

	withdrawn, scopes := nodes.deregistered()
	if len(withdrawn) != len(held) {
		t.Fatalf("deregistered = %v, want every roamer of the partner", withdrawn)
	}

	if len(scopes) != 1 || scopes[0].Mcc != "" || scopes[0].Type != "" {
		t.Fatalf("withdrawal = %+v, want the whole partner", scopes)
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

func TestRemovingAPLMNDeregistersOnlyItsRoamers(t *testing.T) {
	held := []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"},
		{imsi: "208940000000002", regType: db.UERegistrationTypeMME, node: "node-2"},
	}

	env, client, token, partner := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 10*time.Second)

	nodes := startFakeNodes(t, env, partner.ID, held)

	partner.PLMNs = plmnList("208", "93")

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "PUT", env.Server.URL+roamingPartnersPath+"/"+partner.ID, token, partner); err != nil || code != http.StatusOK {
		t.Fatalf("remove a PLMN with roamers: code=%d err=%v (%q), want 200", code, err, resp.Error)
	}

	if withdrawn, scopes := nodes.deregistered(); !slices.Equal(withdrawn, []string{"208940000000002"}) || len(scopes) != 1 || scopes[0].Mcc != "208" || scopes[0].Mnc != "94" {
		t.Fatalf("deregistered %v with %+v, want only the roamer of PLMN 208-94", withdrawn, scopes)
	}

	got, err := env.DB.GetRoamingPartner(context.Background(), partner.ID)
	if err != nil || len(got.PLMNs) != 1 || len(got.Withdrawals) != 0 {
		t.Fatalf("partner after update = %+v, %v, want one PLMN and no withdrawal left", got, err)
	}

	if _, err := env.DB.GetSubscriberOrVisitor(context.Background(), "208930000000001"); err != nil {
		t.Fatalf("the roamer of the kept PLMN was removed: %v", err)
	}
}

func TestRemovingThe4GAgreementDeregistersOnly4GRoamers(t *testing.T) {
	held := []heldRoamer{
		{imsi: "208930000000001", regType: db.UERegistrationTypeMME, node: "node-1"},
		{imsi: "208940000000002", regType: db.UERegistrationTypeAMF3GPPAccess, node: "node-1"},
	}

	env, client, token, partner := roamingPartnerWithRoamers(t, held)
	server.SetRoamerWithdrawalTimeoutForTest(t, 10*time.Second)

	nodes := startFakeNodes(t, env, partner.ID, held)

	partner.DiameterPeerID = ""

	if code, resp, err := apiDo[roamingMessageEnvelope](client, "PUT", env.Server.URL+roamingPartnersPath+"/"+partner.ID, token, partner); err != nil || code != http.StatusOK {
		t.Fatalf("remove the 4G agreement with roamers: code=%d err=%v (%q), want 200", code, err, resp.Error)
	}

	if withdrawn, scopes := nodes.deregistered(); !slices.Equal(withdrawn, []string{"208930000000001"}) || len(scopes) != 1 || scopes[0].Type != db.UERegistrationTypeMME {
		t.Fatalf("deregistered %v with %+v, want only the 4G roamer", withdrawn, scopes)
	}

	if got, err := env.DB.GetRoamingPartner(context.Background(), partner.ID); err != nil || got.DiameterPeerID != nil {
		t.Fatalf("partner after update = %+v, %v, want no 4G agreement", got, err)
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

func TestRoamersCannotRegisterDuringAWithdrawal(t *testing.T) {
	env, _, _, partner := roamingPartnerWithRoamers(t, nil)
	ctx := context.Background()

	withdrawals := []db.RoamingWithdrawal{{PartnerID: partner.ID, Mcc: "208", Mnc: "94"}}

	if err := env.DB.WithdrawRoamers(ctx, partner.ID, withdrawals, time.Minute); err != nil {
		t.Fatalf("WithdrawRoamers: %s", err)
	}

	if _, err := env.DB.RegisterVisitorUE(ctx, "208940000000001", partner.ID, db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrRoamingRefused) {
		t.Fatalf("RegisterVisitorUE in a withdrawn PLMN: %v, want %v", err, db.ErrRoamingRefused)
	}

	if _, err := env.DB.RegisterVisitorUE(ctx, "208930000000001", partner.ID, db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("RegisterVisitorUE in a kept PLMN: %v", err)
	}

	if err := env.DB.ClearRoamingWithdrawals(ctx, partner.ID); err != nil {
		t.Fatalf("ClearRoamingWithdrawals: %s", err)
	}

	if _, err := env.DB.RegisterVisitorUE(ctx, "208940000000001", partner.ID, db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("RegisterVisitorUE after the withdrawal was cleared: %v", err)
	}

	if err := env.DB.WithdrawRoamers(ctx, partner.ID, []db.RoamingWithdrawal{{PartnerID: partner.ID}}, time.Second); err != nil {
		t.Fatalf("WithdrawRoamers: %s", err)
	}

	time.Sleep(2100 * time.Millisecond)

	if _, err := env.DB.RegisterVisitorUE(ctx, "208940000000002", partner.ID, db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("RegisterVisitorUE under an expired withdrawal: %v", err)
	}
}
