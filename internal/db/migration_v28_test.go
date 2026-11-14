// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/dbwriter"
)

func TestMigrationV28KeysRadioEventsByRef(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 27)

	const ref = "eNB:001-01:MacroeNB-00001"

	refValue := ref
	refFilter := &RadioEventFilters{RadioRef: &refValue}

	if err := d.InsertRadioEvent(ctx, &dbwriter.RadioEvent{Timestamp: 1, Protocol: "S1AP", MessageType: "S1SetupRequest", Direction: "inbound", RadioName: "old", RadioRef: ref, Raw: []byte{1}}); err != nil {
		t.Fatalf("insert at v27: %v", err)
	}

	all, total, err := d.ListRadioEvents(ctx, 1, 10, nil)
	if err != nil || total != 1 || all[0].RadioName != "old" || all[0].RadioRef != "" {
		t.Fatalf("list at v27 = %+v (total %d, err %v), want the event without a ref", all, total, err)
	}

	if _, total, err := d.ListRadioEvents(ctx, 1, 10, refFilter); err != nil || total != 0 {
		t.Fatalf("filter by ref at v27: total %d, err %v, want 0", total, err)
	}

	migrateTestDatabase(t, d, 28)

	if err := d.InsertRadioEvent(ctx, &dbwriter.RadioEvent{Timestamp: 2, Protocol: "S1AP", MessageType: "S1SetupResponse", Direction: "outbound", RadioRef: ref, Raw: []byte{2}}); err != nil {
		t.Fatalf("insert at v28: %v", err)
	}

	got, total, err := d.ListRadioEvents(ctx, 1, 10, refFilter)
	if err != nil || total != 1 {
		t.Fatalf("filter by ref at v28: total %d, err %v, want 1", total, err)
	}

	if got[0].MessageType != "S1SetupResponse" || got[0].RadioRef != ref || got[0].RadioName != "" {
		t.Fatalf("event by ref = %+v, want the unnamed radio's S1SetupResponse", got[0])
	}

	byID, err := d.GetRadioEventByID(ctx, got[0].ID)
	if err != nil || byID.RadioRef != ref {
		t.Fatalf("GetRadioEventByID = %+v, %v, want ref %q", byID, err, ref)
	}

	if _, total, err := d.ListRadioEvents(ctx, 1, 10, nil); err != nil || total != 2 {
		t.Fatalf("list at v28: total %d, err %v, want 2", total, err)
	}
}
