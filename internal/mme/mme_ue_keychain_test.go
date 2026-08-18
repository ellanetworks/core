// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/mme/procedure"
)

func TestRemoveUeEndsTheKeyChainOfAConnectionlessUE(t *testing.T) {
	m := newTestMME(t)

	ue := NewUeContext()

	if !ue.BeginKeyChainProc(procedure.S1Handover) {
		t.Fatal("could not claim the key chain")
	}

	m.RemoveUe(ue)

	if !ue.BeginKeyChainProc(procedure.SecurityMode) {
		t.Error("a removed UE still holds its key chain")
	}
}
