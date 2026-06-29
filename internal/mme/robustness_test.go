// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
)

// TestDropStaleUe checks a re-attach reusing the same eNB UE id on the same
// association drops the prior context, so it is not leaked (TS 36.413).
func TestDropStaleUe(t *testing.T) {
	m := newTestMME(t)

	cc := &captureConn{}
	m.NewUe(cc, 7)

	m.DropStaleUe(cc, 7)

	if got := len(m.conns); got != 0 {
		t.Fatalf("DropStaleUe left %d connections, want 0", got)
	}
}

// TestBareConnectionIgnoredByLookups checks that a connection with no bound UE
// context (an Initial UE Message not yet attached) is invisible to UE lookups and
// subscriber counts, and is removed by release.
func TestBareConnectionIgnoredByLookups(t *testing.T) {
	m := newTestMME(t)

	c := m.NewConn(&captureConn{}, 7)

	if c.ue != nil {
		t.Fatal("new connection is not bare")
	}

	if _, ok := m.LookupUe(c.MMEUES1APID); ok {
		t.Fatal("bare connection resolved as a UE")
	}

	if got := m.CountRegisteredSubscribers(); got != 0 {
		t.Fatalf("bare connection counted as a registered subscriber: got %d", got)
	}

	m.ReleaseBareConn(c)

	if got := len(m.conns); got != 0 {
		t.Fatalf("bare connection not removed by release: %d remain", got)
	}
}
