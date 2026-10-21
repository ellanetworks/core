// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
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
