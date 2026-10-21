// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/hss"
)

func TestProfileFilterCriteriaFollowTheServiceOrder(t *testing.T) {
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
	custom := &db.IMSService{
		Name: "custom", Kind: db.IMSServiceKindCustom, ApplicationServerID: otherAS.ID,
		InitialFilterCriteria: []db.IMSInitialFilterCriterion{{
			TriggerPoint:    db.IMSTriggerPoint{ServicePointTriggers: []db.IMSServicePointTrigger{{Groups: []int32{0}, Method: &register}}},
			DefaultHandling: db.IMSDefaultHandlingSessionContinued,
		}},
	}
	mcptt := &db.IMSService{Name: "mcptt", Kind: db.IMSServiceKindMCPTT, ApplicationServerID: mcpttAS.ID}

	for _, svc := range []*db.IMSService{custom, mcptt} {
		if err := database.CreateIMSService(ctx, svc); err != nil {
			t.Fatalf("create service %s: %s", svc.Name, err)
		}
	}

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("get profile: %s", err)
	}

	if err := database.SetProfileIMSServices(ctx, profile.ID, []string{custom.ID, mcptt.ID}); err != nil {
		t.Fatalf("set profile services: %s", err)
	}

	ifcs, err := profileFilterCriteria(ctx, database, profile.ID)
	if err != nil {
		t.Fatalf("profileFilterCriteria: %s", err)
	}

	var priorities []int32

	var servers []string

	for _, ifc := range ifcs {
		priorities = append(priorities, ifc.Priority)
		servers = append(servers, ifc.ApplicationServer.ServerName)
	}

	if !slices.Equal(priorities, []int32{0, 1, 2}) || !slices.Equal(servers, []string{"sip:other.example.org", "sip:mcptt.example.org", "sip:mcptt.example.org"}) {
		t.Fatalf("iFC priorities %v, servers %v; want the custom service first, then the two MCPTT iFCs", priorities, servers)
	}

	none, err := profileFilterCriteria(ctx, database, "unassigned")
	if err != nil || len(none) != 0 {
		t.Fatalf("iFCs of a profile without services = %v, %v", none, err)
	}
}

type recordingPusher struct {
	pushed []string
}

func (r *recordingPusher) PushProfile(_ context.Context, imsi string) error {
	r.pushed = append(r.pushed, imsi)
	return nil
}

func (r *recordingPusher) IdentitiesChanged(_ context.Context, imsi, previousMSISDN string) error {
	r.pushed = append(r.pushed, imsi+" was "+previousMSISDN)
	return nil
}

func TestIMSProfileNotifierCoalescesPushes(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	pusher := &recordingPusher{}
	n := newIMSProfileNotifier(database, pusher)

	n.SubscriberChanged(ctx, "001010000000002")
	n.SubscriberChanged(ctx, "001010000000001")
	n.SubscriberChanged(ctx, "001010000000002")
	n.ProfileChanged(ctx, "no-such-profile")

	for n.step(ctx) {
	}

	if !slices.Equal(pusher.pushed, []string{"001010000000002", "001010000000001"}) {
		t.Fatalf("pushed %v, want each subscriber once, in order", pusher.pushed)
	}

	n.SubscriberChanged(ctx, "001010000000002")

	for n.step(ctx) {
	}

	if !slices.Equal(pusher.pushed, []string{"001010000000002", "001010000000001", "001010000000002"}) {
		t.Fatalf("pushed %v, want a later change pushed again", pusher.pushed)
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
