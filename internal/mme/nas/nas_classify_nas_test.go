// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
)

func TestPlainNonWhitelistedDiscarded(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.Conn().SetSecureExchangeEstablishedForTest(false)

	plain, err := (&eps.AttachComplete{}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), plain)

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatal("a plain non-whitelisted message must be discarded, not processed (TS 24.301 §4.4.4.3)")
	}
}
