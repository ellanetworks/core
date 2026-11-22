// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMigrationV30AddsRadioEventSettings(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtVersion(t, 29)

	enabled, err := d.IsRadioEventsEnabled(ctx)
	if err != nil || !enabled {
		t.Fatalf("IsRadioEventsEnabled at v29 = %t, %v, want true", enabled, err)
	}

	if err := d.UpdateRadioEventSettings(ctx, false); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("UpdateRadioEventSettings at v29: want ErrMigrationPending, got %v", err)
	}

	migrateTestDatabase(t, d, 30)

	enabled, err = d.IsRadioEventsEnabled(ctx)
	if err != nil || !enabled {
		t.Fatalf("IsRadioEventsEnabled at v30 before any update = %t, %v, want true", enabled, err)
	}

	wakeup, stop := d.Changefeed().Wakeup(TopicRadioEventSettings)
	defer stop()

	if err := d.UpdateRadioEventSettings(ctx, false); err != nil {
		t.Fatalf("UpdateRadioEventSettings at v30: %v", err)
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
