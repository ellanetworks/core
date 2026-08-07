// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
)

// TestAbortSessionOwnsByHandle checks that rolling back a partially-created
// session removes only that exact context. A create supersedes the session
// holding the slot before taking it, so a slow first create can find, when its
// own establishment fails, that a second session already owns the key its
// rollback is about to clear (F4).
func TestAbortSessionOwnsByHandle(t *testing.T) {
	s := &SMF{pool: make(map[string]*SMContext), byKey: make(map[string]*SMContext)}

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	const ebi uint8 = 5

	scA, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	// The key is the UE IP lease key, so it is claimed exactly once: a second
	// create for the same slot is refused until the first releases it.
	if _, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil); err == nil {
		t.Fatal("a second session claimed an EPS bearer identity a live session already holds")
	}

	// What a create does instead: supersede the session holding the slot, then
	// take it. scA is now out of the pool but its establishment is still unwinding.
	s.dropFromPool(scA)

	scB, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	if scA.Ref == scB.Ref {
		t.Fatalf("two sessions for the same slot must get distinct refs, got %q twice", scA.Ref)
	}

	// scA has no tunnel or leases, so only the pool removal runs — and it must
	// leave scB, which owns the slot, intact.
	s.abortSession(context.Background(), scA)

	if s.GetSession(scB.Ref) != scB || s.currentEPSSession(supi, ebi) != scB {
		t.Fatalf("abort of a stale context disturbed the live session scB")
	}

	// Aborting the current owner does remove it, from both the pool and the index.
	s.abortSession(context.Background(), scB)

	if s.GetSession(scB.Ref) != nil || s.currentEPSSession(supi, ebi) != nil {
		t.Fatalf("abort of the current context did not remove it")
	}
}
