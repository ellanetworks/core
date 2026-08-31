// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"testing"
)

func TestN3Settings_EndToEnd(t *testing.T) {
	database := setupTestDB(t)

	ctx := context.Background()

	n3Settings, err := database.GetN3Settings(ctx)
	if err != nil {
		t.Fatalf("Couldn't complete GetN3Settings: %s", err)
	}

	if n3Settings.ExternalAddress != "" {
		t.Fatalf("N3 external address should be empty by default")
	}

	newExternalAddress := "1.2.3.4"
	if err := database.UpdateN3Settings(ctx, newExternalAddress); err != nil {
		t.Fatalf("Couldn't Update N3 Settings: %s", err)
	}

	updatedN3Settings, err := database.GetN3Settings(ctx)
	if err != nil {
		t.Fatalf("Couldn't complete GetN3Settings: %s", err)
	}

	if updatedN3Settings.ExternalAddress != newExternalAddress {
		t.Fatalf("N3 external address was not updated correctly")
	}
}
