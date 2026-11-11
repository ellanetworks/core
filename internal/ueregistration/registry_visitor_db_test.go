// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/ueregistration"
	"go.uber.org/zap"
)

const (
	visitorPartnerID = "01890000-0000-7000-8000-0000000000a1"
	visitorPeerID    = "01890000-0000-7000-8000-0000000000b1"
	visitorIMSI      = "208930000000001"
)

func newVisitorDatabase(t *testing.T) *db.Database {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	if err := database.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %s", err)
	}

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	if err := database.CreateRoamingDiameterPeer(ctx, &db.RoamingDiameterPeer{ID: visitorPeerID, DiameterIdentity: "dea.example.org", Address: "10.0.0.7", Port: 3868}); err != nil {
		t.Fatalf("CreateRoamingDiameterPeer: %s", err)
	}

	peer := visitorPeerID

	if err := database.CreateRoamingPartner(ctx, &db.RoamingPartner{
		ID:             visitorPartnerID,
		Name:           "partner",
		ProfileID:      profile.ID,
		DiameterPeerID: &peer,
		PLMNs:          []db.RoamingPLMN{{Mcc: "208", Mnc: "93"}},
	}); err != nil {
		t.Fatalf("CreateRoamingPartner: %s", err)
	}

	return database
}

func TestVisitorRowFollowsTheRegistration(t *testing.T) {
	database := newVisitorDatabase(t)

	r := ueregistration.New(database, "node-a", zap.NewNop())
	r.SetIntervalForTest(time.Hour)

	h := newFakeHolder()
	b := r.Bind(mmeType, amfType, h)

	if _, err := b.RegisterVisitor(context.Background(), visitorIMSI, visitorPartnerID); err != nil {
		t.Fatalf("RegisterVisitor: %s", err)
	}

	visitor, err := database.GetSubscriberOrVisitor(context.Background(), visitorIMSI)
	if err != nil || visitor.RoamingPartnerID == nil || *visitor.RoamingPartnerID != visitorPartnerID {
		t.Fatalf("visitor row = %+v, %v, want one for the partner", visitor, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Run(ctx, nil)

	b.Purge(visitorIMSI)

	waitFor(t, func() bool {
		_, err := database.GetSubscriberOrVisitor(context.Background(), visitorIMSI)
		return errors.Is(err, db.ErrNotFound)
	})
}

func TestVisitorRowStaysWhileTheUEIsHeld(t *testing.T) {
	database := newVisitorDatabase(t)

	r := ueregistration.New(database, "node-a", zap.NewNop())
	h := newFakeHolder()
	b := r.Bind(mmeType, amfType, h)

	if _, err := b.RegisterVisitor(context.Background(), visitorIMSI, visitorPartnerID); err != nil {
		t.Fatalf("RegisterVisitor: %s", err)
	}

	h.mu.Lock()
	h.holds[visitorIMSI] = true
	h.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go r.Run(ctx, nil)

	b.Purge(visitorIMSI)

	time.Sleep(100 * time.Millisecond)

	if _, err := database.GetSubscriberOrVisitor(context.Background(), visitorIMSI); err != nil {
		t.Fatalf("the visitor row of a held UE was removed: %v", err)
	}
}
