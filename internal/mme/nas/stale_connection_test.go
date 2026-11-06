// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestNASOnASupersededConnectionIsDiscarded(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.SetULCountForTest(0)

	old := ue.Conn()

	current := m.NewUeConn(&captureConn{}, 9)
	m.AttachUeConn(context.Background(), ue, current)
	old.BindUeForTest(ue)

	HandleNAS(context.Background(), m, old, protectedUplink(t, ue, nas.MakeCount(0, 0).Value()))

	if ue.ULCount() != 0 {
		t.Fatalf("a NAS message from a superseded connection was processed: ulCount = %d, want 0", ue.ULCount())
	}

	if current.SecureExchangeEstablished() {
		t.Fatal("a NAS message from a superseded connection established secure exchange on the current one")
	}
}
