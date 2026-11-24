// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"
	"testing"
)

func appliedIndexOf(t *testing.T, database *Database) uint64 {
	t.Helper()

	var index uint64
	if err := database.writer().QueryRowContext(t.Context(), "SELECT lastApplied FROM fsm_state WHERE id = 1").Scan(&index); err != nil {
		t.Fatalf("read lastApplied: %v", err)
	}

	return index
}

func TestRegisterUEProposesOnlyWhenTheRegistrationChanges(t *testing.T) {
	database := newInitializedCommandTestDB(t)
	ctx := t.Context()

	const imsi = "001010000000777"

	createTestSubscriber(t, database, imsi)

	register := func(regType, nodeID, cancel string, wantProposed bool) int64 {
		t.Helper()

		before := appliedIndexOf(t, database)

		version, err := database.RegisterUE(ctx, imsi, regType, nodeID, cancel)
		if err != nil {
			t.Fatalf("RegisterUE(%s, %s, %s): %v", regType, nodeID, cancel, err)
		}

		if proposed := appliedIndexOf(t, database) != before; proposed != wantProposed {
			t.Fatalf("RegisterUE(%s, %s, %s): proposed = %v, want %v", regType, nodeID, cancel, proposed, wantProposed)
		}

		return version
	}

	first := register(UERegistrationTypeAMF3GPPAccess, "1", UERegistrationTypeMME, true)

	if again := register(UERegistrationTypeAMF3GPPAccess, "1", UERegistrationTypeMME, false); again != first {
		t.Fatalf("unchanged registration returned version %d, want %d", again, first)
	}

	moved := register(UERegistrationTypeAMF3GPPAccess, "2", UERegistrationTypeMME, true)
	if moved <= first {
		t.Fatalf("moved registration returned version %d, want more than %d", moved, first)
	}

	register(UERegistrationTypeMME, "2", UERegistrationTypeAMF3GPPAccess, true)
	register(UERegistrationTypeAMF3GPPAccess, "2", UERegistrationTypeMME, true)
	register(UERegistrationTypeAMF3GPPAccess, "2", UERegistrationTypeMME, false)

	if err := database.PurgeUERegistration(ctx, imsi, UERegistrationTypeAMF3GPPAccess, "2"); err != nil {
		t.Fatalf("PurgeUERegistration: %v", err)
	}

	register(UERegistrationTypeAMF3GPPAccess, "2", UERegistrationTypeMME, true)

	if err := database.DeleteSubscriber(ctx, imsi); err != nil {
		t.Fatalf("DeleteSubscriber: %v", err)
	}

	before := appliedIndexOf(t, database)

	if _, err := database.RegisterUE(ctx, imsi, UERegistrationTypeAMF3GPPAccess, "2", UERegistrationTypeMME); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RegisterUE after the subscriber is deleted: err = %v, want ErrNotFound", err)
	}

	if appliedIndexOf(t, database) == before {
		t.Fatal("RegisterUE for a deleted subscriber must be proposed so the apply decides")
	}
}
