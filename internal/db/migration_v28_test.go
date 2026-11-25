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

	if _, err := d.PlainDB().ExecContext(ctx,
		"INSERT INTO network_logs (timestamp, protocol, message_type, direction, local_address, remote_address, radio_name, raw, details) VALUES (1, 'S1AP', 'S1SetupRequest', 'inbound', '', '', 'old', x'01', '')"); err != nil {
		t.Fatalf("insert at v27: %v", err)
	}

	migrateTestDatabase(t, d, 28)

	var radioRef string
	if err := d.PlainDB().QueryRowContext(ctx, "SELECT radio_ref FROM network_logs WHERE radio_name = 'old'").Scan(&radioRef); err != nil {
		t.Fatalf("read migrated event: %v", err)
	}

	if radioRef != "" {
		t.Fatalf("migrated event radio_ref = %q, want empty", radioRef)
	}

	migrateTestDatabase(t, d, SchemaVersion())

	if _, total, err := d.ListRadioEvents(ctx, 1, 10, refFilter); err != nil || total != 0 {
		t.Fatalf("filter by ref before inserting a keyed event: total %d, err %v, want 0", total, err)
	}

	if err := d.InsertRadioEvent(ctx, &dbwriter.RadioEvent{Timestamp: 2, Protocol: "S1AP", MessageType: "S1SetupResponse", Direction: "outbound", RadioRef: ref, Raw: []byte{2}}); err != nil {
		t.Fatalf("insert after migrating: %v", err)
	}

	got, total, err := d.ListRadioEvents(ctx, 1, 10, refFilter)
	if err != nil || total != 1 {
		t.Fatalf("filter by ref: total %d, err %v, want 1", total, err)
	}

	if got[0].MessageType != "S1SetupResponse" || got[0].RadioRef != ref || got[0].RadioName != "" {
		t.Fatalf("event by ref = %+v, want the unnamed radio's S1SetupResponse", got[0])
	}

	byID, err := d.GetRadioEventByID(ctx, got[0].ID)
	if err != nil || byID.RadioRef != ref {
		t.Fatalf("GetRadioEventByID = %+v, %v, want ref %q", byID, err, ref)
	}

	all, total, err := d.ListRadioEvents(ctx, 1, 10, nil)
	if err != nil || total != 2 {
		t.Fatalf("list after migrating: total %d, err %v, want 2", total, err)
	}

	if all[1].RadioName != "old" || all[1].RadioRef != "" {
		t.Fatalf("pre-v28 event = %+v, want radio name old without a ref", all[1])
	}
}
