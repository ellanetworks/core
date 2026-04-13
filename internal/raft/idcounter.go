// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
)

// IDCounter provides deterministic, leader-assigned IDs for shared DB tables.
//
// On leader promotion, each counter is seeded from SELECT COALESCE(MAX(id),0)
// for its table. The leader atomically increments the counter before each
// Propose and includes the ID in the command payload. Followers execute the
// INSERT with the explicit ID — no AUTOINCREMENT needed.
//
// Raft guarantees that a new leader sees every committed entry, so re-seeding
// from MAX(id) is safe.
type IDCounter struct {
	counter atomic.Int64
}

// Next returns the next ID, atomically incrementing the counter.
func (c *IDCounter) Next() int64 {
	return c.counter.Add(1)
}

// Seed sets the counter to the given value (typically MAX(id) from the table).
func (c *IDCounter) Seed(current int64) {
	c.counter.Store(current)
}

// IDCounters holds per-table ID counters, seeded on leader promotion.
type IDCounters struct {
	mu       sync.Mutex
	counters map[string]*IDCounter
}

// NewIDCounters creates a new set of ID counters.
func NewIDCounters() *IDCounters {
	return &IDCounters{
		counters: make(map[string]*IDCounter),
	}
}

// tablesWithIDs lists every shared table that uses surrogate integer IDs.
var tablesWithIDs = []string{
	"network_slices",
	"profiles",
	"data_networks",
	"policies",
	"network_rules",
	"subscribers",
	"ip_leases",
	"home_network_keys",
	"users",
	"sessions",
	"api_tokens",
	"bgp_peers",
	"bgp_import_prefixes",
	"routes",
	"retention_policies",
	"audit_logs",
}

// SeedFromDB reads MAX(id) for every shared table and seeds the counters.
// Called on leader promotion.
func (ic *IDCounters) SeedFromDB(ctx context.Context, db *sql.DB) error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	for _, table := range tablesWithIDs {
		var maxID int64

		// #nosec: G201 — table names are compile-time constants from tablesWithIDs
		row := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", table))
		if err := row.Scan(&maxID); err != nil {
			return fmt.Errorf("seed ID counter for %s: %w", table, err)
		}

		counter, ok := ic.counters[table]
		if !ok {
			counter = &IDCounter{}
			ic.counters[table] = counter
		}

		counter.Seed(maxID)
	}

	return nil
}

// Next returns the next ID for the given table. Panics if the table has not
// been seeded (programming error — means the table is missing from
// tablesWithIDs).
func (ic *IDCounters) Next(table string) int64 {
	ic.mu.Lock()
	counter, ok := ic.counters[table]
	ic.mu.Unlock()

	if !ok {
		panic(fmt.Sprintf("IDCounters.Next called for unseeded table %q", table))
	}

	return counter.Next()
}
