// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
)

func ueWithSessions(t *testing.T, ids ...uint8) *amf.UeContext {
	t.Helper()

	ue := amf.NewUeContext()
	for _, id := range ids {
		if err := ue.CreateSmContext(id, "ref", nil, "internet"); err != nil {
			t.Fatalf("CreateSmContext(%d): %v", id, err)
		}
	}

	return ue
}

func TestAllocateEPSBearerIdentityStartsAtFive(t *testing.T) {
	ue := ueWithSessions(t, 1, 2)

	first, err := ue.AllocateEPSBearerIdentity(1)
	if err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	second, err := ue.AllocateEPSBearerIdentity(2)
	if err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	if first != 5 || second != 6 {
		t.Fatalf("allocated (%d, %d), want (5, 6)", first, second)
	}
}

func TestAllocateEPSBearerIdentityIsStable(t *testing.T) {
	ue := ueWithSessions(t, 3)

	first, err := ue.AllocateEPSBearerIdentity(3)
	if err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	again, err := ue.AllocateEPSBearerIdentity(3)
	if err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	if again != first {
		t.Fatalf("re-allocation returned %d, want the assigned %d", again, first)
	}
}

func TestEPSBearerIdentityIsReleasedWithTheSession(t *testing.T) {
	ue := ueWithSessions(t, 1, 2)

	if _, err := ue.AllocateEPSBearerIdentity(1); err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	ue.DeleteSmContext(1)

	if _, ok := ue.EPSBearerIdentity(1); ok {
		t.Fatal("a released session must hold no EPS bearer identity")
	}

	reused, err := ue.AllocateEPSBearerIdentity(2)
	if err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	if reused != 5 {
		t.Fatalf("allocated %d, want the released 5", reused)
	}
}

func TestAllocateEPSBearerIdentityExhaustion(t *testing.T) {
	ue := amf.NewUeContext()

	for id := uint8(1); id <= 11; id++ {
		if _, err := ue.AllocateEPSBearerIdentity(id); err != nil {
			t.Fatalf("AllocateEPSBearerIdentity(%d): %v", id, err)
		}
	}

	if _, err := ue.AllocateEPSBearerIdentity(12); !errors.Is(err, amf.ErrNoEPSBearerIdentity) {
		t.Fatalf("error = %v, want ErrNoEPSBearerIdentity", err)
	}
}

func TestEPSBearerIdentityAllocatesBeforeTheSessionExists(t *testing.T) {
	ue := amf.NewUeContext()

	ebi, err := ue.AllocateEPSBearerIdentity(1)
	if err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	if ebi != 5 {
		t.Fatalf("allocated %d, want 5", ebi)
	}

	// Releasing the session releases its identity: the range is 5..15, so leaking
	// one per session would exhaust it.
	ue.DeleteSmContext(1)

	if _, ok := ue.EPSBearerIdentity(1); ok {
		t.Fatal("a released identity must not remain assigned")
	}
}

func TestEPSBearerIdentities(t *testing.T) {
	ue := ueWithSessions(t, 1, 2, 3)

	for _, id := range []uint8{1, 3} {
		if _, err := ue.AllocateEPSBearerIdentity(id); err != nil {
			t.Fatalf("AllocateEPSBearerIdentity(%d): %v", id, err)
		}
	}

	got := ue.EPSBearerIdentities()
	if len(got) != 2 || got[1] != 5 || got[3] != 6 {
		t.Fatalf("EPSBearerIdentities = %v, want {1:5, 3:6}", got)
	}
}
