// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf_test

import (
	"context"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

type switchingStore struct {
	mu sync.Mutex
	id string
}

func (s *switchingStore) GetSessionPolicy(context.Context, string, int32, string, string) (*db.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return &db.Policy{ID: s.id}, nil
}

type recordingEnforcer struct {
	notified []*smf.PolicyDecision
}

func (r *recordingEnforcer) UpdateNotify(_ context.Context, _ string, d *smf.PolicyDecision) error {
	r.notified = append(r.notified, d)

	return nil
}

func TestReconcilePushesAChangedDecision(t *testing.T) {
	ctx := context.Background()
	store := &switchingStore{id: "first"}
	enforcer := &recordingEnforcer{}

	p := pcf.New(store, zap.NewNop())
	p.SetEnforcer(enforcer)

	created, err := p.CreateAssociation(ctx, "session", smf.PolicyContext{Supi: supi(t, "001010000000001"), Dnn: models.IMSDataNetworkName, Subscribed: smf.SubscribedQoS{Var5qi: 5, Arp: 1}})
	if err != nil {
		t.Fatalf("CreateAssociation: %v", err)
	}

	p.Reconcile(ctx)

	if len(enforcer.notified) != 0 {
		t.Fatalf("an unchanged decision was pushed: %+v", enforcer.notified)
	}

	store.mu.Lock()
	store.id = "second"
	store.mu.Unlock()

	p.Reconcile(ctx)

	if len(enforcer.notified) != 1 {
		t.Fatalf("pushed %d decisions, want 1", len(enforcer.notified))
	}

	d := enforcer.notified[0]
	if d.PolicyID != "second" || d.Var5qi != 5 || d.Revision <= created.Revision {
		t.Fatalf("pushed %+v, want policy second with the subscribed 5QI at a later revision", d)
	}

	p.TerminateAssociation("session")
	p.Reconcile(ctx)

	if len(enforcer.notified) != 1 {
		t.Fatal("a decision was pushed for a terminated association")
	}
}
