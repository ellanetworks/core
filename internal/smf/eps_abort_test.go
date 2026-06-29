// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
)

// TestAbortSessionOwnsByHandle checks that rolling back a partially-created
// session removes only that exact context from the pool. If a concurrent create
// has already replaced the (IMSI,EBI) entry, the rollback must leave the live
// session intact rather than tearing down the second call's session (F4).
func TestAbortSessionOwnsByHandle(t *testing.T) {
	s := &SMF{pool: make(map[string]*SMContext)}

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	const ebi uint8 = 5

	ref := CanonicalName(supi, ebi)

	scA := s.NewSession(supi, ebi, "internet", nil) // first create
	scB := s.NewSession(supi, ebi, "internet", nil) // second create overwrites the entry

	if s.GetSession(ref) != scB {
		t.Fatalf("expected pool to hold the second context after overwrite")
	}

	// Roll back the first (failed) create. scA has no tunnel or leases, so only
	// the pool removal runs — and it must be a no-op because scB owns the entry.
	s.abortSession(context.Background(), scA)

	if got := s.GetSession(ref); got != scB {
		t.Fatalf("abort of a stale context removed the live session: got %v, want scB", got)
	}

	// Aborting the current owner does remove it.
	s.abortSession(context.Background(), scB)

	if got := s.GetSession(ref); got != nil {
		t.Fatalf("abort of the current context did not remove it: got %v", got)
	}
}
