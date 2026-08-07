// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
)

func TestNewSessionRefusesAClaimedKey(t *testing.T) {
	s := &SMF{pool: make(map[string]*SMContext), byKey: make(map[string]*SMContext)}

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	const ebi uint8 = 5

	live, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil); err == nil {
		t.Error("second session for the same EPS bearer identity = nil error, want a refusal")
	}

	if s.currentEPSSession(supi, ebi) != live {
		t.Errorf("current session for EBI %d = another instance, want the live %q", ebi, live.Ref)
	}

	if got := len(s.pool); got != 1 {
		t.Errorf("pool holds %d sessions, want only the live one", got)
	}

	// A duplicate UE-allocated identity is dropped (TS 23.502 §4.11.1.1 NOTE 5).
	fiveG, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: 3}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	degraded, err := s.NewSession(supi, Access4G, SessionIdentity{PDUSessionID: 3, EBI: ebi + 1}, "internet", nil)
	if err != nil {
		t.Fatalf("NewSession with a duplicate PDU session identity: %v", err)
	}

	if degraded.PDUSessionID != 0 || degraded.EBI != ebi+1 {
		t.Errorf("identity = pdu-session-id %d ebi %d, want 0 and %d", degraded.PDUSessionID, degraded.EBI, ebi+1)
	}

	if s.currentPDUSession(supi, 3) != fiveG {
		t.Errorf("current session for PDU session identity 3 = another instance, want the 5G %q", fiveG.Ref)
	}

	// A 5G session has no other identity to fall back on.
	if _, err := s.NewSession(supi, Access5G, SessionIdentity{PDUSessionID: 3}, "internet", nil); err == nil {
		t.Error("second 5G session for the same PDU session identity = nil error, want a refusal")
	}
}

// Rolling back a partially-created session removes that exact context and no
// other, so a later create that has taken over the slot survives it.
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

	// Supersede the slot, releasing the key before the replacement claims it.
	s.dropFromPool(scA)

	scB, err := s.NewSession(supi, Access4G, SessionIdentity{EBI: ebi}, "internet", nil)
	if err != nil {
		t.Fatal(err)
	}

	if scA.Ref == scB.Ref {
		t.Fatalf("refs of two sessions for the same (SUPI, EBI) = %q twice, want distinct", scA.Ref)
	}

	if s.currentEPSSession(supi, ebi) != scB {
		t.Fatalf("current session for EBI %d = another instance, want the replacement %q", ebi, scB.Ref)
	}

	// scA has no tunnel or leases, so only the pool removal runs.
	s.abortSession(context.Background(), scA)

	if s.GetSession(scB.Ref) != scB || s.currentEPSSession(supi, ebi) != scB {
		t.Fatalf("aborting the stale context left %q out of the pool or the index, want it in both", scB.Ref)
	}

	s.abortSession(context.Background(), scB)

	if s.GetSession(scB.Ref) != nil || s.currentEPSSession(supi, ebi) != nil {
		t.Fatalf("aborting %q left it in the pool or the index, want it removed from both", scB.Ref)
	}
}
