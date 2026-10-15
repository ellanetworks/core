// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"testing"
)

func TestPCSCFAddressesReadEmptyBeforeV23(t *testing.T) {
	d := newDatabaseAtVersion(t, 22)

	addresses, err := d.ListPCSCFAddresses(context.Background())
	if err != nil {
		t.Fatalf("ListPCSCFAddresses: %v", err)
	}

	if len(addresses) != 0 {
		t.Fatalf("addresses = %v, want none before v23", addresses)
	}
}
