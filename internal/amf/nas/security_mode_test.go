// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
)

// TestSecurityMode_BlockedByConflict verifies the security mode procedure is
// claimed before the security context is mutated: while an N2 handover holds the
// key-changing mutual exclusion, the re-key is refused and no NAS keys are
// derived (TS 33.501 §6.9.5.1).
func TestSecurityMode_BlockedByConflict(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("build UE and radio: %v", err)
	}

	conn := ue.NasConn()
	if conn == nil {
		t.Fatal("UE has no NAS connection")
	}

	// An in-flight N2 handover holds the key-changing mutual exclusion.
	if _, err := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.N2Handover}); err != nil {
		t.Fatalf("start N2 handover: %v", err)
	}

	before := ue.KnasEncForTest()

	if err := securityMode(context.Background(), amf.New(nil, nil, nil), ue); err == nil {
		t.Fatal("security mode must be refused while a handover holds the key chain")
	}

	if conn.Procedures.Active(procedure.SecurityMode) {
		t.Fatal("security mode must not be claimed when blocked by a handover")
	}

	if ue.KnasEncForTest() != before {
		t.Fatal("a blocked security mode must not derive the NAS keys")
	}
}
