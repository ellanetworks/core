// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
)

// TestPlainNonWhitelistedDiscarded asserts handleNAS drops a plain EMM message
// whose type is not on the §4.4.4.3 whitelist.
func TestPlainNonWhitelistedDiscarded(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	// An un-established connection so the plain whitelist (not the secure-exchange
	// discard) is what gates the message.
	ue.Conn().SetSecureExchangeEstablishedForTest(false)

	// A plain ATTACH COMPLETE is not on the whitelist; it must be discarded, so the
	// UE remains EMM-REGISTERED and is not driven by an unauthenticated msg.
	plain, err := (&eps.AttachComplete{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), plain)

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatal("a plain non-whitelisted message must be discarded, not processed (TS 24.301 §4.4.4.3)")
	}
}
