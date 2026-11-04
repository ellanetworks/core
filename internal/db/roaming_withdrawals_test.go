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

func TestWithdrawingAPartnerRefusesItsRoamers(t *testing.T) {
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.registerVisitor(t, visitorIMSI)

	ctx := context.Background()

	if err := f.db.SetRoamingPartnerWithdrawing(ctx, "unknown", time.Now().Add(time.Minute)); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("withdraw an unknown partner: %v, want %v", err, db.ErrNotFound)
	}

	if err := f.db.SetRoamingPartnerWithdrawing(ctx, roamingPartnerID, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("SetRoamingPartnerWithdrawing: %s", err)
	}

	if p, err := f.db.GetRoamingPartner(ctx, roamingPartnerID); err != nil || !p.Withdrawing(time.Now()) {
		t.Fatalf("partner = %+v, %v, want it withdrawing", p, err)
	}

	if v, err := f.db.GetVisitor(ctx, visitorIMSI); err != nil || !v.Withdrawing(time.Now()) || v.Deregistering(time.Now()) {
		t.Fatalf("visitor = %+v, %v, want its partner withdrawing", v, err)
	}

	for _, imsi := range []string{visitorIMSI, "208930000000002"} {
		if _, err := f.db.RegisterVisitorUE(ctx, imsi, roamingPartnerID, db.UERegistrationTypeAMF3GPPAccess, "node-1", ""); !errors.Is(err, db.ErrRoamingRefused) {
			t.Fatalf("register %s during the withdrawal: %v, want %v", imsi, err, db.ErrRoamingRefused)
		}
	}

	if n, err := f.db.CountVisitorsOfPartner(ctx, roamingPartnerID); err != nil || n != 1 {
		t.Fatalf("CountVisitorsOfPartner = %d, %v, want the attached roamer", n, err)
	}

	if err := f.db.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if n, err := f.db.CountVisitorsOfPartner(ctx, roamingPartnerID); err != nil || n != 0 {
		t.Fatalf("CountVisitorsOfPartner = %d, %v, want none left", n, err)
	}

	if err := f.db.SetRoamingPartnerWithdrawing(ctx, roamingPartnerID, time.Time{}); err != nil {
		t.Fatalf("clear the withdrawal: %s", err)
	}

	f.registerVisitor(t, visitorIMSI)

	if err := f.db.SetRoamingPartnerWithdrawing(ctx, roamingPartnerID, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("SetRoamingPartnerWithdrawing in the past: %s", err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000002", roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("register after the withdrawal expired: %s", err)
	}
}

func TestDeregisteringARoamerRefusesOnlyThatRoamer(t *testing.T) {
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.registerVisitor(t, visitorIMSI)
	f.registerVisitor(t, "208930000000002")

	ctx := context.Background()

	if err := f.db.SetVisitorDeregistering(ctx, "208930000000003", time.Now().Add(time.Minute)); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("deregister an unknown roamer: %v, want %v", err, db.ErrNotFound)
	}

	if err := f.db.SetVisitorDeregistering(ctx, visitorIMSI, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("SetVisitorDeregistering: %s", err)
	}

	if v, err := f.db.GetVisitor(ctx, visitorIMSI); err != nil || !v.Deregistering(time.Now()) || v.Withdrawing(time.Now()) {
		t.Fatalf("visitor = %+v, %v, want it deregistering", v, err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, db.UERegistrationTypeAMF3GPPAccess, "node-1", ""); !errors.Is(err, db.ErrRoamerDeregistering) {
		t.Fatalf("register the deregistering roamer: %v, want %v", err, db.ErrRoamerDeregistering)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000002", roamingPartnerID, db.UERegistrationTypeAMF3GPPAccess, "node-1", ""); err != nil {
		t.Fatalf("register another roamer: %s", err)
	}

	if err := f.db.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if _, err := f.db.GetVisitor(ctx, visitorIMSI); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("deregistered roamer: %v, want it deleted", err)
	}

	f.registerVisitor(t, visitorIMSI)
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
