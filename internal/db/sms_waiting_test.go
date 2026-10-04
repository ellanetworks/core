// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

const (
	waitingIMSI = "001010000000001"
	scA         = "15550000000"
	scB         = "15550000001"
)

func setupWaitingTestDB(t *testing.T) *db.Database {
	t.Helper()

	database, profileID := setupMSISDNTestDB(t)

	if err := database.CreateSubscriber(context.Background(), newMSISDNSubscriber(waitingIMSI, "15551230001", profileID)); err != nil {
		t.Fatalf("CreateSubscriber: %s", err)
	}

	return database
}

func waitingOf(t *testing.T, database *db.Database) *db.SMSWaiting {
	t.Helper()

	w, err := database.GetSMSWaiting(context.Background(), waitingIMSI)
	if errors.Is(err, db.ErrNotFound) {
		return nil
	}

	if err != nil {
		t.Fatalf("GetSMSWaiting: %s", err)
	}

	return w
}

func TestSMSWaitingRecordsFlagsAndServiceCentres(t *testing.T) {
	ctx := context.Background()
	database := setupWaitingTestDB(t)

	if w := waitingOf(t, database); w != nil {
		t.Fatalf("waiting = %+v on a fresh subscriber", w)
	}

	if err := database.RecordSMSWaiting(ctx, db.SMSWaitingUpdate{IMSI: waitingIMSI, ServiceCentre: scA, MemoryFull: true}); err != nil {
		t.Fatalf("RecordSMSWaiting: %s", err)
	}

	if err := database.RecordSMSWaiting(ctx, db.SMSWaitingUpdate{IMSI: waitingIMSI, ServiceCentre: scB}); err != nil {
		t.Fatalf("RecordSMSWaiting: %s", err)
	}

	if err := database.RecordSMSWaiting(ctx, db.SMSWaitingUpdate{IMSI: waitingIMSI, ServiceCentre: scA}); err != nil {
		t.Fatalf("RecordSMSWaiting: %s", err)
	}

	w := waitingOf(t, database)
	if w == nil || !w.MemoryFull || !slices.Equal(w.ServiceCentres, []string{scA, scB}) {
		t.Fatalf("waiting = %+v, want the memory-full flag kept and both service centres listed once", w)
	}
}

func TestSMSWaitingClearsMemoryFullOnly(t *testing.T) {
	ctx := context.Background()
	database := setupWaitingTestDB(t)

	if err := database.RecordSMSWaiting(ctx, db.SMSWaitingUpdate{IMSI: waitingIMSI, ServiceCentre: scA, MemoryFull: true}); err != nil {
		t.Fatalf("RecordSMSWaiting: %s", err)
	}

	if err := database.ClearSMSMemoryFull(ctx, waitingIMSI); err != nil {
		t.Fatalf("ClearSMSMemoryFull: %s", err)
	}

	if w := waitingOf(t, database); w == nil || w.MemoryFull || !slices.Equal(w.ServiceCentres, []string{scA}) {
		t.Fatalf("waiting = %+v, want the memory-full flag cleared and the service centre kept", w)
	}
}

func TestSMSWaitingRemovesOneServiceCentreThenTheEntry(t *testing.T) {
	ctx := context.Background()
	database := setupWaitingTestDB(t)

	for _, sc := range []string{scA, scB} {
		if err := database.RecordSMSWaiting(ctx, db.SMSWaitingUpdate{IMSI: waitingIMSI, ServiceCentre: sc}); err != nil {
			t.Fatalf("RecordSMSWaiting: %s", err)
		}
	}

	if err := database.RemoveSMSWaitingCentre(ctx, waitingIMSI, scA); err != nil {
		t.Fatalf("RemoveSMSWaitingCentre: %s", err)
	}

	if w := waitingOf(t, database); w == nil || w.MemoryFull || !slices.Equal(w.ServiceCentres, []string{scB}) {
		t.Fatalf("waiting = %+v, want the other service centre kept and the flag cleared", w)
	}

	if err := database.RemoveSMSWaitingCentre(ctx, waitingIMSI, scB); err != nil {
		t.Fatalf("RemoveSMSWaitingCentre: %s", err)
	}

	if w := waitingOf(t, database); w != nil {
		t.Fatalf("waiting = %+v after its last service centre was removed", w)
	}
}

func TestSMSWaitingIsDeletedWithTheSubscriber(t *testing.T) {
	ctx := context.Background()
	database := setupWaitingTestDB(t)

	if err := database.RecordSMSWaiting(ctx, db.SMSWaitingUpdate{IMSI: waitingIMSI, ServiceCentre: scA}); err != nil {
		t.Fatalf("RecordSMSWaiting: %s", err)
	}

	if err := database.DeleteSubscriber(ctx, waitingIMSI); err != nil {
		t.Fatalf("DeleteSubscriber: %s", err)
	}

	if w := waitingOf(t, database); w != nil {
		t.Fatalf("waiting = %+v after the subscriber was deleted", w)
	}
}

func TestSMSWaitingForAnUnknownSubscriber(t *testing.T) {
	database := setupWaitingTestDB(t)

	err := database.RecordSMSWaiting(context.Background(), db.SMSWaitingUpdate{IMSI: "001019999999999", ServiceCentre: scA})
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("RecordSMSWaiting = %v, want not found", err)
	}
}

func TestStaleSMSWaitingIsSwept(t *testing.T) {
	ctx := context.Background()
	database := setupWaitingTestDB(t)

	if err := database.RecordSMSWaiting(ctx, db.SMSWaitingUpdate{IMSI: waitingIMSI, ServiceCentre: scA}); err != nil {
		t.Fatalf("RecordSMSWaiting: %s", err)
	}

	if err := database.DeleteStaleSMSWaiting(ctx, time.Hour); err != nil {
		t.Fatalf("DeleteStaleSMSWaiting: %s", err)
	}

	if w := waitingOf(t, database); w == nil {
		t.Fatal("a fresh entry was swept")
	}

	if err := database.DeleteStaleSMSWaiting(ctx, -time.Hour); err != nil {
		t.Fatalf("DeleteStaleSMSWaiting: %s", err)
	}

	if w := waitingOf(t, database); w != nil {
		t.Fatalf("waiting = %+v after the sweep", w)
	}
}
