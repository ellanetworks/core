// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/hss"
)

func mcpttIFCs(asID string) []db.IMSInitialFilterCriterion {
	register, originating := "REGISTER", db.IMSSessionCaseOriginatingRegistered
	header, content := "^P-Asserted-Service$", `^urn:urn-7:3gpp-service\.ims\.icsi\.mcptt$`

	return []db.IMSInitialFilterCriterion{
		{
			ApplicationServerID:     asID,
			Trigger:                 &db.IMSTrigger{ConditionsMatch: db.IMSConditionsMatchAll, Groups: []db.IMSConditionGroup{{Conditions: []db.IMSCondition{{Method: &register}}}}},
			DefaultHandling:         new(db.IMSDefaultHandlingSessionContinued),
			IncludeRegisterRequest:  true,
			IncludeRegisterResponse: true,
		},
		{
			ApplicationServerID: asID,
			Trigger: &db.IMSTrigger{
				ConditionsMatch: db.IMSConditionsMatchAll,
				Groups: []db.IMSConditionGroup{{Conditions: []db.IMSCondition{
					{SessionCase: &originating},
					{SIPHeader: &db.IMSHeaderMatch{Header: header, Content: &content}},
				}}},
			},
			DefaultHandling: new(db.IMSDefaultHandlingSessionTerminated),
		},
	}
}

func TestSubscriberFilterCriteriaKeepTheirOrder_TS29228_B22(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	mcpttAS := &db.IMSApplicationServer{Name: "mcptt", ServerName: "sip:mcptt.example.org"}
	otherAS := &db.IMSApplicationServer{Name: "other", ServerName: "sip:other.example.org"}

	for _, as := range []*db.IMSApplicationServer{mcpttAS, otherAS} {
		if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
			t.Fatalf("create application server %s: %s", as.Name, err)
		}
	}

	register := "REGISTER"
	other := db.IMSInitialFilterCriterion{
		ApplicationServerID: otherAS.ID,
		Trigger:             &db.IMSTrigger{ConditionsMatch: db.IMSConditionsMatchAll, Groups: []db.IMSConditionGroup{{Conditions: []db.IMSCondition{{Method: &register}}}}},
		DefaultHandling:     new(db.IMSDefaultHandlingSessionContinued),
	}

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("get profile: %s", err)
	}

	if err := database.SetProfileIMSFilterCriteria(ctx, profile.ID, append([]db.IMSInitialFilterCriterion{other}, mcpttIFCs(mcpttAS.ID)...)); err != nil {
		t.Fatalf("set profile iFCs: %s", err)
	}

	subscriber := &db.Subscriber{
		Imsi: "001010000000001", SequenceNumber: "000000000001", PermanentKey: "6f30087629feb0b089783c81d0ae09b5",
		Opc: "21a7e1897dfb481d62439142cdf1b6ee", ProfileID: profile.ID,
	}
	if err := database.CreateSubscriber(ctx, subscriber); err != nil {
		t.Fatalf("create subscriber: %s", err)
	}

	ifcs, err := database.ListSubscriberIMSFilterCriteria(ctx, subscriber.Imsi)
	if err != nil {
		t.Fatalf("ListSubscriberIMSFilterCriteria: %s", err)
	}

	var priorities []int32

	var servers []string

	for _, ifc := range ifcs {
		priorities = append(priorities, ifc.Priority)
		servers = append(servers, ifc.ApplicationServer.ServerName)
	}

	if !slices.Equal(priorities, []int32{0, 1, 2}) || !slices.Equal(servers, []string{"sip:other.example.org", "sip:mcptt.example.org", "sip:mcptt.example.org"}) {
		t.Fatalf("iFC priorities %v, servers %v; want the other server's iFC first, then the two MCPTT iFCs", priorities, servers)
	}

	none, err := database.ListSubscriberIMSFilterCriteria(ctx, "001010000000002")
	if err != nil || len(none) != 0 {
		t.Fatalf("iFCs of an unknown subscriber = %v, %v", none, err)
	}
}

type recordingReconciler struct {
	mu         sync.Mutex
	reconciled []string
}

func (r *recordingReconciler) Reconcile(_ context.Context, imsi string) error {
	r.mu.Lock()
	r.reconciled = append(r.reconciled, imsi)
	r.mu.Unlock()

	return nil
}

func (r *recordingReconciler) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.reconciled)
}

func imsNotifierDatabase(t *testing.T, registrations map[string]string) *db.Database {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("get profile: %s", err)
	}

	for imsi, host := range registrations {
		if err := database.CreateSubscriber(ctx, &db.Subscriber{
			Imsi: imsi, SequenceNumber: "000000000001", PermanentKey: "6f30087629feb0b089783c81d0ae09b5",
			Opc: "21a7e1897dfb481d62439142cdf1b6ee", ProfileID: profile.ID,
		}); err != nil {
			t.Fatalf("create subscriber: %s", err)
		}

		if host == "" {
			continue
		}

		reg := &db.IMSRegistration{IMSI: imsi, State: db.IMSRegistered, ServerName: "sip:" + host, OriginHost: host, OriginRealm: "ims.example.org"}
		if _, swapped, err := database.CompareAndSwapIMSRegistration(ctx, imsi, nil, reg); err != nil || !swapped {
			t.Fatalf("store the IMS registration of %s: swapped %t, %v", imsi, swapped, err)
		}
	}

	return database
}

func TestIMSProfileNotifierReconcilesEachSubscriberOnce(t *testing.T) {
	ctx := context.Background()
	database := imsNotifierDatabase(t, map[string]string{
		"001010000000001": "scscf.example.org",
		"001010000000002": "scscf.example.org",
		"001010000000003": "",
	})

	reconciler := &recordingReconciler{}
	n := newIMSProfileNotifier(database, reconciler)

	n.SubscriberChanged(ctx, "001010000000002")
	n.SubscriberChanged(ctx, "001010000000001")
	n.SubscriberChanged(ctx, "001010000000002")
	n.ProfileChanged(ctx, "no-such-profile")

	for n.step(ctx) {
	}

	if got := reconciler.seen(); !slices.Equal(got, []string{"001010000000002", "001010000000001"}) {
		t.Fatalf("reconciled %v, want each subscriber once, in order", got)
	}

	n.SubscriberChanged(ctx, "001010000000002")
	n.AllSubscribersChanged(ctx)

	for n.step(ctx) {
	}

	if got := reconciler.seen()[2:]; !slices.Equal(got, []string{"001010000000002", "001010000000001"}) {
		t.Fatalf("reconciled %v after the next changes, want each registered subscriber once more", got)
	}
}

func TestServiceIdentityLookup(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	for _, as := range []*db.IMSApplicationServer{
		{Name: "mcptt", ServerName: "sip:MCPTT.example.org", PublicServiceIdentities: []string{"sip:!.*!@mcptt.example.org"}},
		{Name: "conf", ServerName: "sip:conf.example.org", PublicServiceIdentities: []string{"sip:factory@mcptt.example.org", "tel:+1-555-0199"}},
	} {
		if err := database.CreateIMSApplicationServer(ctx, as); err != nil {
			t.Fatalf("create application server %s: %s", as.Name, err)
		}
	}

	adapter := &hssDBAdapter{db: database}

	for identity, want := range map[string]*hss.ServiceIdentity{
		"sip:group-1@mcptt.example.org": {ServerName: "sip:mcptt.example.org", Wildcard: "sip:!.*!@mcptt.example.org"},
		"sip:factory@MCPTT.example.org": {ServerName: "sip:conf.example.org"},
		"tel:+15550199":                 {ServerName: "sip:conf.example.org"},
		"sip:group-1@example.org":       nil,
	} {
		got, err := adapter.ServiceIdentity(ctx, identity)
		if err != nil {
			t.Fatalf("ServiceIdentity(%s): %s", identity, err)
		}

		if (got == nil) != (want == nil) || (got != nil && *got != *want) {
			t.Fatalf("ServiceIdentity(%s) = %+v, want %+v", identity, got, want)
		}
	}
}
