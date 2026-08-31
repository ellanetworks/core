// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func protectedUplink(t *testing.T, ue *mme.UeContext, count uint32) []byte {
	t.Helper()

	plain := []byte{0x07, 0x60, 0x00}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.Count(count), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

// TS 24.301 §4.4.3
func TestNASUplinkReplayRejected(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.SetULCountForTest(0)

	msg := protectedUplink(t, ue, nas.MakeCount(0, 0).Value())

	HandleNAS(context.Background(), m, ue.Conn(), msg)

	if ue.ULCount() != 1 {
		t.Fatalf("valid message not accepted: ulCount = %d, want 1", ue.ULCount())
	}

	HandleNAS(context.Background(), m, ue.Conn(), msg)

	if ue.ULCount() != 1 {
		t.Fatalf("replay accepted: ulCount advanced to %d", ue.ULCount())
	}
}

// TS 24.301 §4.4.3.6
func TestNASUplinkCountWrap(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.SetULCountForTest(255)

	HandleNAS(context.Background(), m, ue.Conn(), protectedUplink(t, ue, nas.MakeCount(0, 255).Value()))

	if ue.ULCount() != 256 {
		t.Fatalf("sequence 255 not accepted: ulCount = %d, want 256", ue.ULCount())
	}

	HandleNAS(context.Background(), m, ue.Conn(), protectedUplink(t, ue, nas.MakeCount(1, 0).Value()))

	if ue.ULCount() != 257 {
		t.Fatalf("wrapped message not accepted: ulCount = %d, want 257", ue.ULCount())
	}
}
