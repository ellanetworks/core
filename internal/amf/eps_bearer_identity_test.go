// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
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

func assignEBI(t *testing.T, ue *amf.UeContext, pduSessionID uint8) error {
	t.Helper()

	ebi, err := ue.NextEPSBearerIdentity(pduSessionID)
	if err != nil {
		return err
	}

	ue.SetEPSBearerIdentity(pduSessionID, ebi)

	return nil
}

func TestEPSBearerIdentityStartsAtFive(t *testing.T) {
	ue := ueWithSessions(t, 1, 2)

	if err := assignEBI(t, ue, 1); err != nil {
		t.Fatalf("assign for session 1: %v", err)
	}

	if err := assignEBI(t, ue, 2); err != nil {
		t.Fatalf("assign for session 2: %v", err)
	}

	first, _ := ue.EPSBearerIdentity(1)
	second, _ := ue.EPSBearerIdentity(2)

	if first != 5 || second != 6 {
		t.Fatalf("assigned (%d, %d), want (5, 6)", first, second)
	}
}

func TestEPSBearerIdentityIsStable(t *testing.T) {
	ue := ueWithSessions(t, 3)

	if err := assignEBI(t, ue, 3); err != nil {
		t.Fatalf("assign: %v", err)
	}

	first, _ := ue.EPSBearerIdentity(3)

	again, err := ue.NextEPSBearerIdentity(3)
	if err != nil {
		t.Fatalf("NextEPSBearerIdentity: %v", err)
	}

	if again != first {
		t.Fatalf("re-assignment returned %d, want the assigned %d", again, first)
	}
}

func TestEPSBearerIdentityIsReleasedWithTheSession(t *testing.T) {
	ue := ueWithSessions(t, 1, 2)

	if err := assignEBI(t, ue, 1); err != nil {
		t.Fatalf("assign: %v", err)
	}

	ue.DeleteSmContext(1)

	if _, ok := ue.EPSBearerIdentity(1); ok {
		t.Fatal("a released session must hold no EPS bearer identity")
	}

	reused, err := ue.NextEPSBearerIdentity(2)
	if err != nil {
		t.Fatalf("NextEPSBearerIdentity: %v", err)
	}

	if reused != 5 {
		t.Fatalf("assigned %d, want the released 5", reused)
	}
}

func TestEPSBearerIdentitiesAreReleasedWithTheSessionsInBulk(t *testing.T) {
	ue := ueWithSessions(t, 1, 2, 3)

	for _, id := range []uint8{1, 2, 3} {
		if err := assignEBI(t, ue, id); err != nil {
			t.Fatalf("assign for session %d: %v", id, err)
		}
	}

	ue.ClearRegistrationData(context.Background())

	if got := ue.EPSBearerIdentities(); len(got) != 0 {
		t.Fatalf("EPS bearer identities survived the release: %v", got)
	}
}

func TestEPSBearerIdentityExhaustion(t *testing.T) {
	ue := ueWithSessions(t, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11)

	for id := uint8(1); id <= 11; id++ {
		if err := assignEBI(t, ue, id); err != nil {
			t.Fatalf("assign for session %d: %v", id, err)
		}
	}

	if err := ue.CreateSmContext(12, "ref", nil, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := ue.NextEPSBearerIdentity(12); !errors.Is(err, amf.ErrNoEPSBearerIdentity) {
		t.Fatalf("error = %v, want ErrNoEPSBearerIdentity", err)
	}
}

func TestEPSBearerIdentityIsNotRecordedWithoutASession(t *testing.T) {
	ue := amf.NewUeContext()

	ebi, err := ue.NextEPSBearerIdentity(1)
	if err != nil {
		t.Fatalf("NextEPSBearerIdentity: %v", err)
	}

	if ebi != 5 {
		t.Fatalf("picked %d, want 5", ebi)
	}

	ue.SetEPSBearerIdentity(1, ebi)

	if _, ok := ue.EPSBearerIdentity(1); ok {
		t.Fatal("an identity was recorded for a session that was never created")
	}
}

func TestEPSBearerIdentities(t *testing.T) {
	ue := ueWithSessions(t, 1, 2, 3)

	for _, id := range []uint8{1, 3} {
		if err := assignEBI(t, ue, id); err != nil {
			t.Fatalf("assign for session %d: %v", id, err)
		}
	}

	got := ue.EPSBearerIdentities()
	if len(got) != 2 || got[1] != 5 || got[3] != 6 {
		t.Fatalf("EPSBearerIdentities = %v, want {1:5, 3:6}", got)
	}
}
