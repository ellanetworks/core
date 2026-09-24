// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"
	"time"
)

func TestUnansweredInitialContextSetupReleasesARegisteredUEsConnection(t *testing.T) {
	ue, _ := newDownlinkOrderUE(t)
	ue.ForceStateForTest(Registered)

	conn := ue.Conn()
	a := conn.amf
	a.ICSGuardCfg.ExpireTime = 5 * time.Millisecond

	conn.MarkICSPending()
	conn.superviseICS(t.Context())

	deadline := time.Now().Add(time.Second)
	for !a.ReleaseClaimed(conn) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if !a.ReleaseClaimed(conn) {
		t.Fatal("no UE Context Release Command for an Initial Context Setup the gNB never answered")
	}

	if ue.State() != Registered {
		t.Fatalf("state = %s, want Registered: only the NG connection is released", ue.State())
	}
}

func TestAnsweredInitialContextSetupStopsItsSupervision(t *testing.T) {
	ue, _ := newDownlinkOrderUE(t)

	conn := ue.Conn()
	conn.MarkICSPending()
	conn.superviseICS(t.Context())

	if !conn.icsGuard.Active() {
		t.Fatal("Initial Context Setup is not supervised")
	}

	conn.MarkICSCompleted()

	if conn.icsGuard.Active() {
		t.Fatal("an answered Initial Context Setup is still supervised")
	}
}
