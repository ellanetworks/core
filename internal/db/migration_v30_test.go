// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"testing"
	"time"
)

func TestMigrationV30AddsRadioEventSettings(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 29)

	if got := countRows(t, d.PlainDB(), "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = '"+RadioEventSettingsTableName+"'"); got != 0 {
		t.Fatalf("radio event settings tables at v29 = %d, want 0", got)
	}

	migrateTestDatabase(t, d, 30)

	if got := countRows(t, d.PlainDB(), "SELECT COUNT(*) FROM "+RadioEventSettingsTableName); got != 0 {
		t.Fatalf("radio event settings rows at v30 = %d, want 0", got)
	}

	migrateTestDatabase(t, d, SchemaVersion())

	enabled, err := d.IsRadioEventsEnabled(ctx)
	if err != nil || !enabled {
		t.Fatalf("IsRadioEventsEnabled before any update = %t, %v, want true", enabled, err)
	}

	wakeup, stop := d.Changefeed().Wakeup(TopicRadioEventSettings)
	defer stop()

	if err := d.UpdateRadioEventSettings(ctx, false); err != nil {
		t.Fatalf("UpdateRadioEventSettings: %v", err)
	}

	select {
	case <-wakeup:
	case <-time.After(5 * time.Second):
		t.Fatal("no radio event settings wakeup after the update")
	}

	enabled, err = d.IsRadioEventsEnabled(ctx)
	if err != nil || enabled {
		t.Fatalf("IsRadioEventsEnabled after disabling = %t, %v, want false", enabled, err)
	}

	if err := d.UpdateRadioEventSettings(ctx, true); err != nil {
		t.Fatalf("UpdateRadioEventSettings re-enable: %v", err)
	}

	enabled, err = d.IsRadioEventsEnabled(ctx)
	if err != nil || !enabled {
		t.Fatalf("IsRadioEventsEnabled after re-enabling = %t, %v, want true", enabled, err)
	}
}
