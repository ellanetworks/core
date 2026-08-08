// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
)

// TS 23.502 §4.11.2.3 step 10 runs the EPS bearer deactivation except steps 4-7,
// and step 4c is the S1AP leg: the UE has left E-UTRAN, so nothing is signalled
// for the bearer. Only the last PDN connection going takes the context with it.
func TestSessionDropped(t *testing.T) {
	t.Run("not the last PDN: nothing is signalled and the UE stays attached", func(t *testing.T) {
		m := newTestMME(t)
		ue, cc := securedUE(t, m)

		p := testPDN(ue)
		p.SessionRef = "imsi-001010000000001-3#1"

		ue.EnsurePDN(6)

		sent := len(cc.sent)

		m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, p.SessionRef)

		if m.LookupPDN(ue, DefaultERABID) != nil {
			t.Error("the moved PDN connection was not dropped")
		}

		if m.LookupPDN(ue, 6) == nil {
			t.Error("a PDN connection that did not move was dropped")
		}

		if len(cc.sent) != sent {
			t.Errorf("sent %d S1AP messages, want 0: step 10 excludes the S1AP leg", len(cc.sent)-sent)
		}

		if ue.EMMState() == EMMDeregistered {
			t.Error("the UE was detached though it still holds a PDN connection")
		}
	})

	t.Run("the last PDN: the context goes with it", func(t *testing.T) {
		m := newTestMME(t)
		ue, _ := securedUE(t, m)

		p := testPDN(ue)
		p.SessionRef = "imsi-001010000000001-3#1"

		m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, p.SessionRef)

		if ue.EMMState() != EMMDeregistered {
			t.Error("an attached UE was left with no PDN connection, which it cannot be served in")
		}
	})

	t.Run("a stale ref names another instance and is ignored", func(t *testing.T) {
		m := newTestMME(t)
		ue, _ := securedUE(t, m)

		p := testPDN(ue)
		p.SessionRef = "imsi-001010000000001-3#2"

		m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, "imsi-001010000000001-3#1")

		if m.LookupPDN(ue, DefaultERABID) == nil {
			t.Error("a report for a superseded session dropped the current one")
		}
	})
}
