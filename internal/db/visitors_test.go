// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
)

func TestSubscriberCapIsEnforcedInTheApply(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.db.SetMaxSubscribers(2)

	f.createOwnSubscriber(t, "001010000000001")
	f.registerVisitor(t, visitorIMSI)

	err := f.db.CreateSubscriber(ctx, &db.Subscriber{Imsi: "001010000000002", SequenceNumber: "000000000001", PermanentKey: "00112233445566778899aabbccddeeff", Opc: "00112233445566778899aabbccddeeff", ProfileID: f.profileID})
	if !errors.Is(err, db.ErrSubscriberLimit) {
		t.Fatalf("home subscriber over the cap: err = %v, want ErrSubscriberLimit", err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, "208930000000002", roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrSubscriberLimit) {
		t.Fatalf("visitor over the cap: err = %v, want ErrSubscriberLimit", err)
	}

	if _, err := f.db.GetSubscriberOrVisitor(ctx, "208930000000002"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("refused visitor was stored: %v", err)
	}

	if _, err := f.db.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, db.UERegistrationTypeAMF3GPPAccess, "node-1", db.UERegistrationTypeMME); err != nil {
		t.Fatalf("existing visitor at the cap: %s", err)
	}
}

func TestVisitorStateFollowsTheVisitor(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.registerVisitor(t, visitorIMSI)
	f.createOwnSubscriber(t, "001010000000001")

	dn, err := f.db.GetDataNetwork(ctx, db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	for _, imsi := range []string{visitorIMSI, "001010000000001"} {
		if _, err := f.db.AllocateIPLease(ctx, dn.ID, "ipv4", imsi, 1, "node-1"); err != nil {
			t.Fatalf("AllocateIPLease for %s: %s", imsi, err)
		}
	}

	if _, err := f.db.AllocateIPLease(ctx, dn.ID, "ipv4", "208930000000077", 1, "node-1"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("AllocateIPLease for an unknown IMSI: err = %v, want ErrNotFound", err)
	}

	if _, err := f.db.RegisterUE(ctx, "208930000000077", db.UERegistrationTypeMME, "node-1", ""); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("RegisterUE for an unknown IMSI: err = %v, want ErrNotFound", err)
	}

	report := func(imsi string) *dbwriter.FlowReport {
		return &dbwriter.FlowReport{SubscriberID: imsi, SourceIP: "10.45.0.1", DestinationIP: "8.8.8.8", Protocol: 17, Packets: 1, Bytes: 80, StartTime: 1, EndTime: 2, Direction: "uplink"}
	}

	if err := f.db.InsertFlowReports(ctx, []*dbwriter.FlowReport{report(visitorIMSI), report("001010000000001"), report("208930000000077")}); err != nil {
		t.Fatalf("InsertFlowReports: %s", err)
	}

	if _, total, err := f.db.ListFlowReports(ctx, 1, 10, nil); err != nil || total != 2 {
		t.Fatalf("flow reports = %d, %v; want the visitor's and the subscriber's, not the unknown IMSI's", total, err)
	}

	if err := f.db.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if deleted, err := f.db.DeleteVisitorIfUnregistered(ctx, visitorIMSI); err != nil || !deleted {
		t.Fatalf("DeleteVisitorIfUnregistered = %v, %v", deleted, err)
	}

	if n, err := f.db.CountLeasesByIMSI(ctx, visitorIMSI); err != nil || n != 0 {
		t.Fatalf("leases of a deleted visitor = %d, %v", n, err)
	}

	if _, err := f.db.GetUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("registration of a deleted visitor: err = %v, want ErrNotFound", err)
	}

	if _, total, err := f.db.ListFlowReports(ctx, 1, 10, nil); err != nil || total != 2 {
		t.Fatalf("flow reports after the visitor left = %d, %v; want them kept until retention", total, err)
	}

	if err := f.db.DeleteSubscriber(ctx, "001010000000001"); err != nil {
		t.Fatalf("DeleteSubscriber: %s", err)
	}

	if n, err := f.db.CountLeasesByIMSI(ctx, "001010000000001"); err != nil || n != 0 {
		t.Fatalf("leases of a deleted subscriber = %d, %v", n, err)
	}

	if _, total, err := f.db.ListFlowReports(ctx, 1, 10, nil); err != nil || total != 2 {
		t.Fatalf("flow reports after the subscriber was deleted = %d, %v; want them kept until retention", total, err)
	}
}

func TestExpiredRoamingWithdrawalsAreSwept(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.registerVisitor(t, visitorIMSI)

	if err := f.db.WithdrawRoamer(ctx, visitorIMSI, 0); err == nil {
		t.Fatal("withdrawal without a lifetime accepted")
	}

	if err := f.db.WithdrawRoamer(ctx, visitorIMSI, time.Second); err != nil {
		t.Fatalf("WithdrawRoamer: %s", err)
	}

	p, err := f.db.GetRoamingPartner(ctx, roamingPartnerID)
	if err != nil || len(p.Withdrawals) != 1 || p.Withdrawals[0].ExpiresAt <= time.Now().Unix()-1 {
		t.Fatalf("withdrawals = %+v, %v", p, err)
	}

	if n, err := f.db.DeleteExpiredRoamingWithdrawals(ctx); err != nil || n != 0 {
		t.Fatalf("DeleteExpiredRoamingWithdrawals of a live withdrawal = %d, %v", n, err)
	}

	time.Sleep(2100 * time.Millisecond)

	if n, err := f.db.DeleteExpiredRoamingWithdrawals(ctx); err != nil || n != 1 {
		t.Fatalf("DeleteExpiredRoamingWithdrawals = %d, %v; want the expired roamer withdrawal", n, err)
	}

	if p, err := f.db.GetRoamingPartner(ctx, roamingPartnerID); err != nil || len(p.Withdrawals) != 0 {
		t.Fatalf("withdrawals after the sweep = %+v, %v", p, err)
	}
}

func TestRenewGeneratedSEPPIdentityIsConditional(t *testing.T) {
	ctx := context.Background()
	f := newRoamingFixture(t)

	if _, err := f.db.EnsureSEPPIdentity(ctx, 1); err != nil {
		t.Fatalf("EnsureSEPPIdentity: %s", err)
	}

	original, err := f.db.GetSEPPIdentity(ctx, 1)
	if err != nil {
		t.Fatalf("GetSEPPIdentity: %s", err)
	}

	renewed, err := f.db.RenewGeneratedSEPPIdentity(ctx, 1, original.CertChainPEM)
	if err != nil || renewed.CertChainPEM == original.CertChainPEM {
		t.Fatalf("RenewGeneratedSEPPIdentity = %v", err)
	}

	if _, err := f.db.RenewGeneratedSEPPIdentity(ctx, 1, original.CertChainPEM); !errors.Is(err, db.ErrSEPPIdentityChanged) {
		t.Fatalf("renew from a stale chain: err = %v, want ErrSEPPIdentityChanged", err)
	}

	imported := ownSEPPIdentity(t, "1")
	if err := f.db.PutSEPPIdentity(ctx, &imported); err != nil {
		t.Fatalf("PutSEPPIdentity: %s", err)
	}

	if _, err := f.db.RenewGeneratedSEPPIdentity(ctx, 1, renewed.CertChainPEM); !errors.Is(err, db.ErrSEPPIdentityChanged) {
		t.Fatalf("renew over an import: err = %v, want ErrSEPPIdentityChanged", err)
	}

	if got, err := f.db.GetSEPPIdentity(ctx, 1); err != nil || got.Source != db.SEPPIdentityImported || got.CertChainPEM != imported.CertChainPEM {
		t.Fatalf("imported identity overwritten: %+v, %v", got, err)
	}
}

func TestVisitorWritesReplicate(t *testing.T) {
	ctx := context.Background()
	database := setupRaftTestDB(t)

	createSubscriber(t, database, "001010000000001")

	own, err := database.GetSubscriber(ctx, "001010000000001")
	if err != nil {
		t.Fatalf("GetSubscriber: %s", err)
	}

	partner := &db.RoamingPartner{ID: roamingPartnerID, Name: "partner", PLMNs: plmns("208", "93"), ProfileID: own.ProfileID}
	if err := database.CreateRoamingPartner(ctx, partner); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	if _, err := database.RegisterVisitorUE(ctx, visitorIMSI, roamingPartnerID, db.UERegistrationTypeMME, "node-1", ""); err != nil {
		t.Fatalf("RegisterVisitorUE: %s", err)
	}

	expectOneRaftEntry(t, database, "SetVisitorMSISDN", func() error { return database.SetVisitorMSISDN(ctx, visitorIMSI, "33612345678") })
	expectOneRaftEntry(t, database, "WithdrawRoamer", func() error { return database.WithdrawRoamer(ctx, visitorIMSI, time.Minute) })

	if n, err := database.DeleteExpiredRoamingWithdrawals(ctx); err != nil || n != 0 {
		t.Fatalf("DeleteExpiredRoamingWithdrawals = %d, %v; want the live withdrawal kept", n, err)
	}

	report := &dbwriter.FlowReport{SubscriberID: "001010000000001", SourceIP: "10.45.0.1", DestinationIP: "8.8.8.8", Protocol: 17, Packets: 1, Bytes: 80, StartTime: 1, EndTime: 2, Direction: "uplink"}
	if err := database.InsertFlowReports(ctx, []*dbwriter.FlowReport{report}); err != nil {
		t.Fatalf("InsertFlowReports: %s", err)
	}

	if err := database.DeleteSubscriber(ctx, "001010000000001"); err != nil {
		t.Fatalf("DeleteSubscriber: %s", err)
	}

	if _, total, err := database.ListFlowReports(ctx, 1, 10, nil); err != nil || total != 1 {
		t.Fatalf("flow reports of a deleted subscriber = %d, %v; want them kept until retention", total, err)
	}

	if err := database.PurgeUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME, "node-1"); err != nil {
		t.Fatalf("PurgeUERegistration: %s", err)
	}

	if n, err := database.DeleteUnregisteredVisitors(ctx); err != nil || n != 1 {
		t.Fatalf("DeleteUnregisteredVisitors = %d, %v", n, err)
	}

	if _, err := database.GetUERegistration(ctx, visitorIMSI, db.UERegistrationTypeMME); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("registration of a deleted visitor: err = %v, want ErrNotFound", err)
	}
}
