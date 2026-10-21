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

func TestUpdateAssociationRevisesOnlyAChangedDecision(t *testing.T) {
	ctx := context.Background()
	store := &switchingStore{id: "first"}
	subscribed := smf.SubscribedQoS{Var5qi: 5, Arp: models.Arp{PriorityLevel: 1}}

	p := pcf.New(store, zap.NewNop())

	created, err := p.CreateAssociation(ctx, "session", smf.PolicyContext{Supi: supi(t, "001010000000001"), Dnn: models.IMSDataNetworkName, Subscribed: subscribed})
	if err != nil {
		t.Fatalf("CreateAssociation: %v", err)
	}

	same, err := p.UpdateAssociation(ctx, "session", subscribed)
	if err != nil {
		t.Fatalf("UpdateAssociation: %v", err)
	}

	if same != nil {
		t.Fatalf("decision %+v for an unchanged policy, want none so the SMF keeps the one it holds", same)
	}

	store.mu.Lock()
	store.id = "second"
	store.mu.Unlock()

	changed, err := p.UpdateAssociation(ctx, "session", subscribed)
	if err != nil {
		t.Fatalf("UpdateAssociation: %v", err)
	}

	if changed.PolicyID != "second" || changed.Revision <= created.Revision {
		t.Fatalf("decision %+v, want policy second at a later revision", changed)
	}
}
