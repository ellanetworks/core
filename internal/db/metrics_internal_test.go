// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsCollectorOmitsFailedReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	database, err := NewDatabaseWithoutRaft(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	if err := database.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	if err := os.Remove(dbPath); err != nil {
		t.Fatalf("remove database file: %v", err)
	}

	reg := prometheus.NewPedanticRegistry()

	canary := prometheus.NewGauge(prometheus.GaugeOpts{Name: "canary", Help: "canary"})
	canary.Set(1)

	reg.MustRegister(canary)
	reg.MustRegister(newMetricsCollector(database))

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather returned an error, which would blank the whole endpoint: %v", err)
	}

	got := make(map[string]bool, len(families))
	for _, f := range families {
		got[f.GetName()] = true
	}

	for _, name := range []string{
		"app_database_storage_bytes",
		"app_ip_addresses_total",
		"app_ip_addresses_allocated_total",
	} {
		if got[name] {
			t.Errorf("%s was published from a failed read; it must be absent, not zero", name)
		}
	}

	if !got["canary"] {
		t.Error("unrelated metric was suppressed by the failing collector")
	}
}

func TestMetricsCollectorPublishesHealthyReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	database, err := NewDatabaseWithoutRaft(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}()

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(newMetricsCollector(database))

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	got := make(map[string]bool, len(families))
	for _, f := range families {
		got[f.GetName()] = true
	}

	for _, name := range []string{
		"app_database_storage_bytes",
		"app_ip_addresses_total",
		"app_ip_addresses_allocated_total",
	} {
		if !got[name] {
			t.Errorf("%s missing from a healthy database", name)
		}
	}
}
