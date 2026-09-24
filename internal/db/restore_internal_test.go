// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"
	"testing"
)

func TestRestoreRefusedWhileAnotherRestoreHoldsTheLock(t *testing.T) {
	database := &Database{}

	database.restoreMu.Lock()
	defer database.restoreMu.Unlock()

	if err := database.Restore(t.Context(), nil); !errors.Is(err, ErrRestoreInProgress) {
		t.Fatalf("Restore err = %v, want ErrRestoreInProgress", err)
	}

	if err := database.SelfRestore(t.Context()); !errors.Is(err, ErrRestoreInProgress) {
		t.Fatalf("SelfRestore err = %v, want ErrRestoreInProgress", err)
	}
}
