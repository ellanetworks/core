// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

// TestLocalOnlySingletons_SeededOnFreshDB pins the contract documented
// above localOnlyTables in changeset_replication.go: every singleton
// local-only settings table is seeded with its documented default at
// NewDatabase time on every node, so readers never observe an empty
// table. A freshly-started HA follower depends on this; without it the
// daemon crashes on startup when its consumer hits sql.ErrNoRows.
//
// Operator-set values must survive a restart unchanged. The
// "preserves_operator_value" sub-test asserts the idempotency property
// that backs that guarantee.
//
// When adding a new singleton local-only table, add a sub-test here
// covering the default it seeds.
func TestLocalOnlySingletons_SeededOnFreshDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabaseWithoutRaft(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() {
		if database != nil {
			if err := database.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		}
	})

	ctx := context.Background()

	t.Run("nat", func(t *testing.T) {
		got, err := database.IsNATEnabled(ctx)
		if err != nil {
			t.Fatalf("IsNATEnabled on fresh DB returned error: %v", err)
		}

		if got != db.NATDefaultEnabled {
			t.Fatalf("IsNATEnabled = %v, want default %v", got, db.NATDefaultEnabled)
		}
	})

	t.Run("flow_accounting", func(t *testing.T) {
		got, err := database.IsFlowAccountingEnabled(ctx)
		if err != nil {
			t.Fatalf("IsFlowAccountingEnabled on fresh DB returned error: %v", err)
		}

		if got != db.FlowAccountingDefaultEnabled {
			t.Fatalf("IsFlowAccountingEnabled = %v, want default %v", got, db.FlowAccountingDefaultEnabled)
		}
	})

	t.Run("n3", func(t *testing.T) {
		got, err := database.GetN3Settings(ctx)
		if err != nil {
			t.Fatalf("GetN3Settings on fresh DB returned error: %v", err)
		}

		if got == nil {
			t.Fatal("GetN3Settings returned nil settings")
		}

		if got.ExternalAddress != db.N3DefaultExternalAddress {
			t.Fatalf("ExternalAddress = %q, want default %q", got.ExternalAddress, db.N3DefaultExternalAddress)
		}
	})

	t.Run("bgp", func(t *testing.T) {
		got, err := database.GetBGPSettings(ctx)
		if err != nil {
			t.Fatalf("GetBGPSettings on fresh DB returned error: %v", err)
		}

		if got == nil {
			t.Fatal("GetBGPSettings returned nil settings")
		}

		if got.Enabled != db.BGPDefaultEnabled {
			t.Fatalf("Enabled = %v, want default %v", got.Enabled, db.BGPDefaultEnabled)
		}

		if got.LocalAS != db.BGPDefaultLocalAS {
			t.Fatalf("LocalAS = %d, want default %d", got.LocalAS, db.BGPDefaultLocalAS)
		}

		if got.RouterID != db.BGPDefaultRouterID {
			t.Fatalf("RouterID = %q, want default %q", got.RouterID, db.BGPDefaultRouterID)
		}

		if got.ListenAddress != db.BGPDefaultListenAddress {
			t.Fatalf("ListenAddress = %q, want default %q", got.ListenAddress, db.BGPDefaultListenAddress)
		}
	})

	// Restart preserves operator state. Switch every singleton away from
	// its default, close the DB, reopen it, and confirm the seed step
	// did not overwrite the operator-set values.
	t.Run("preserves_operator_value_across_restart", func(t *testing.T) {
		if err := database.UpdateNATSettings(ctx, !db.NATDefaultEnabled); err != nil {
			t.Fatalf("UpdateNATSettings: %v", err)
		}

		if err := database.UpdateFlowAccountingSettings(ctx, !db.FlowAccountingDefaultEnabled); err != nil {
			t.Fatalf("UpdateFlowAccountingSettings: %v", err)
		}

		if err := database.UpdateN3Settings(ctx, "10.0.0.1"); err != nil {
			t.Fatalf("UpdateN3Settings: %v", err)
		}

		if err := database.UpdateBGPSettings(ctx, &db.BGPSettings{
			Enabled:       !db.BGPDefaultEnabled,
			LocalAS:       65000,
			RouterID:      "10.0.0.1",
			ListenAddress: "10.0.0.1:179",
		}); err != nil {
			t.Fatalf("UpdateBGPSettings: %v", err)
		}

		if err := database.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		database = nil

		reopened, err := db.NewDatabaseWithoutRaft(ctx, dbPath)
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}

		t.Cleanup(func() {
			if err := reopened.Close(); err != nil {
				t.Fatalf("reopen Close: %v", err)
			}
		})

		nat, err := reopened.IsNATEnabled(ctx)
		if err != nil {
			t.Fatalf("IsNATEnabled after restart: %v", err)
		}

		if nat != !db.NATDefaultEnabled {
			t.Fatalf("NAT was reset to default after restart: got %v", nat)
		}

		fa, err := reopened.IsFlowAccountingEnabled(ctx)
		if err != nil {
			t.Fatalf("IsFlowAccountingEnabled after restart: %v", err)
		}

		if fa != !db.FlowAccountingDefaultEnabled {
			t.Fatalf("flow accounting was reset to default after restart: got %v", fa)
		}

		n3, err := reopened.GetN3Settings(ctx)
		if err != nil {
			t.Fatalf("GetN3Settings after restart: %v", err)
		}

		if n3.ExternalAddress != "10.0.0.1" {
			t.Fatalf("N3 external_address was reset after restart: got %q", n3.ExternalAddress)
		}

		bgp, err := reopened.GetBGPSettings(ctx)
		if err != nil {
			t.Fatalf("GetBGPSettings after restart: %v", err)
		}

		if bgp.LocalAS != 65000 || bgp.RouterID != "10.0.0.1" || bgp.Enabled == db.BGPDefaultEnabled {
			t.Fatalf("BGP settings were reset after restart: %+v", bgp)
		}
	})
}
