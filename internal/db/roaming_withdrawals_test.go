// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func TestRoamingWithdrawalsFor(t *testing.T) {
	peer := "peer"
	fqdn := "sepp.5gc.mnc093.mcc208.3gppnetwork.org"

	existing := db.RoamingPartner{ID: "p", PLMNs: plmns("208", "93", "208", "94"), DiameterPeerID: &peer, SEPPFQDN: &fqdn}

	if got := db.RoamingWithdrawalsFor(existing, nil); len(got) != 1 || got[0] != (db.RoamingWithdrawal{PartnerID: "p"}) {
		t.Fatalf("delete withdrawals = %+v, want the whole partner", got)
	}

	updated := existing
	updated.PLMNs = plmns("208", "93")
	updated.DiameterPeerID = nil
	updated.SEPPFQDN = nil

	want := []db.RoamingWithdrawal{
		{PartnerID: "p", Mcc: "208", Mnc: "94"},
		{PartnerID: "p", Type: db.UERegistrationTypeAMF3GPPAccess},
		{PartnerID: "p", Type: db.UERegistrationTypeMME},
	}

	got := db.RoamingWithdrawalsFor(existing, &updated)
	if len(got) != len(want) {
		t.Fatalf("update withdrawals = %+v, want %+v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("update withdrawals = %+v, want %+v", got, want)
		}
	}

	if got := db.RoamingWithdrawalsFor(existing, &existing); len(got) != 0 {
		t.Fatalf("withdrawals of an unchanged partner = %+v, want none", got)
	}
}

func TestRoamingWithdrawalCovers(t *testing.T) {
	now := time.Now()
	live := now.Add(time.Minute).Unix()

	for _, tc := range []struct {
		name string
		w    db.RoamingWithdrawal
		imsi string
		typ  string
		want bool
	}{
		{name: "partner", w: db.RoamingWithdrawal{ExpiresAt: live}, imsi: "208930000000001", typ: db.UERegistrationTypeMME, want: true},
		{name: "PLMN", w: db.RoamingWithdrawal{Mcc: "208", Mnc: "93", ExpiresAt: live}, imsi: "208930000000001", typ: db.UERegistrationTypeMME, want: true},
		{name: "other PLMN", w: db.RoamingWithdrawal{Mcc: "208", Mnc: "94", ExpiresAt: live}, imsi: "208930000000001", typ: db.UERegistrationTypeMME},
		{name: "RAT", w: db.RoamingWithdrawal{Type: db.UERegistrationTypeMME, ExpiresAt: live}, imsi: "208930000000001", typ: db.UERegistrationTypeMME, want: true},
		{name: "other RAT", w: db.RoamingWithdrawal{Type: db.UERegistrationTypeAMF3GPPAccess, ExpiresAt: live}, imsi: "208930000000001", typ: db.UERegistrationTypeMME},
		{name: "expired", w: db.RoamingWithdrawal{ExpiresAt: now.Unix()}, imsi: "208930000000001", typ: db.UERegistrationTypeMME},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.w.Covers(tc.imsi, tc.typ, now); got != tc.want {
				t.Fatalf("Covers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWithdrawRoamers(t *testing.T) {
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.registerVisitor(t, visitorIMSI)

	ctx := context.Background()

	if err := f.db.WithdrawRoamers(ctx, roamingPartnerID, []db.RoamingWithdrawal{{PartnerID: roamingPartnerID, Mcc: "208", Mnc: "95"}}, time.Minute); !errors.Is(err, db.ErrRoamingConflict) {
		t.Fatalf("withdraw a PLMN the partner does not hold: %v, want %v", err, db.ErrRoamingConflict)
	}

	if err := f.db.WithdrawRoamers(ctx, "unknown", nil, time.Minute); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("withdraw the roamers of an unknown partner: %v, want %v", err, db.ErrNotFound)
	}

	if err := f.db.WithdrawRoamers(ctx, roamingPartnerID, []db.RoamingWithdrawal{{PartnerID: roamingPartnerID, Type: db.UERegistrationTypeMME}}, time.Minute); err != nil {
		t.Fatalf("WithdrawRoamers: %s", err)
	}

	if n, err := f.db.CountWithdrawnRoamers(ctx, roamingPartnerID); err != nil || n != 1 {
		t.Fatalf("CountWithdrawnRoamers = %d, %v, want the attached 4G roamer", n, err)
	}

	if w, err := f.db.RoamingWithdrawalOf(ctx, visitorIMSI, db.UERegistrationTypeMME); err != nil || !w.AgreementOnly() {
		t.Fatalf("RoamingWithdrawalOf = %+v, %v, want the 4G agreement withdrawal", w, err)
	}

	if _, err := f.db.RoamingWithdrawalOf(ctx, visitorIMSI, db.UERegistrationTypeAMF3GPPAccess); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("RoamingWithdrawalOf over 5G: %v, want not found", err)
	}

	if err := f.db.WithdrawRoamers(ctx, roamingPartnerID, []db.RoamingWithdrawal{{PartnerID: roamingPartnerID}}, time.Minute); err != nil {
		t.Fatalf("WithdrawRoamers: %s", err)
	}

	p, err := f.db.GetRoamingPartner(ctx, roamingPartnerID)
	if err != nil || len(p.Withdrawals) != 1 || p.Withdrawals[0].Type != "" {
		t.Fatalf("withdrawals = %+v, %v, want the last withdrawal to replace the previous one", p, err)
	}

	if err := f.db.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if n, err := f.db.CountWithdrawnRoamers(ctx, roamingPartnerID); err != nil || n != 1 {
		t.Fatalf("CountWithdrawnRoamers = %d, %v, want the visitor row counted until it is deleted", n, err)
	}

	if _, err := f.db.DeleteVisitorIfUnregistered(ctx, visitorIMSI); err != nil {
		t.Fatalf("DeleteVisitorIfUnregistered: %s", err)
	}

	if n, err := f.db.CountWithdrawnRoamers(ctx, roamingPartnerID); err != nil || n != 0 {
		t.Fatalf("CountWithdrawnRoamers = %d, %v, want none left", n, err)
	}

	if err := f.db.DeleteRoamingPartner(ctx, roamingPartnerID); err != nil {
		t.Fatalf("DeleteRoamingPartner: %s", err)
	}
}

func TestVisitorRegistrationsOfNode(t *testing.T) {
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.createOwnSubscriber(t, "001010000000001")

	ctx := context.Background()

	if _, err := f.db.RegisterUE(ctx, "001010000000001", db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	f.registerVisitor(t, visitorIMSI)

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000002", roamingPartnerID, db.UERegistrationTypeMME, "node-2", ""); err != nil {
		t.Fatalf("RegisterVisitorUE: %s", err)
	}

	regs, err := f.db.ListVisitorRegistrationsOfNode(ctx, "node-1")
	if err != nil || len(regs) != 1 || regs[0].Imsi != visitorIMSI {
		t.Fatalf("ListVisitorRegistrationsOfNode = %+v, %v, want only node-1's visitor", regs, err)
	}
}

func TestDeletingAClusterMemberRemovesItsVisitors(t *testing.T) {
	f := newRoamingFixture(t)
	f.createPartner(t)

	ctx := context.Background()

	for _, nodeID := range []string{"node-1", "node-2"} {
		if err := f.db.UpsertClusterMember(ctx, &db.ClusterMember{NodeID: nodeID, APIAddress: "10.0.0.1:8443"}); err != nil {
			t.Fatalf("UpsertClusterMember: %s", err)
		}
	}

	f.registerVisitor(t, visitorIMSI)

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000002", roamingPartnerID, db.UERegistrationTypeMME, "node-2", ""); err != nil {
		t.Fatalf("RegisterVisitorUE: %s", err)
	}

	if err := f.db.DeleteClusterMember(ctx, "node-1"); err != nil {
		t.Fatalf("DeleteClusterMember: %s", err)
	}

	if _, err := f.db.GetSubscriberOrVisitor(ctx, visitorIMSI); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("visitor of the removed node: %v, want it deleted", err)
	}

	if _, err := f.db.GetSubscriberOrVisitor(ctx, "208930000000002"); err != nil {
		t.Fatalf("visitor of another node: %v, want it kept", err)
	}
}
