// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/udm"
)

func TestAusfDBAdapterAdvanceSequenceNumberUnknownSubscriber(t *testing.T) {
	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	adapter := &ausfDBAdapter{db: database}

	_, err = adapter.AdvanceSequenceNumber(ctx, "001019999999999", "", "")
	if err == nil {
		t.Fatal("AdvanceSequenceNumber succeeded for an unprovisioned subscriber")
	}

	if !errors.Is(err, udm.ErrSubscriberUnknown) {
		t.Errorf("error %v does not wrap udm.ErrSubscriberUnknown", err)
	}

	if !errors.Is(err, db.ErrNotFound) {
		t.Errorf("error %v no longer wraps db.ErrNotFound", err)
	}
}
