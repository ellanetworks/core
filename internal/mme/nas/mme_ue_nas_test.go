// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

func TestVerifiedMessageMarksSecureExchange(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.Conn().SetSecureExchangeEstablishedForTest(false) // fresh connection, not yet established
	ue.SetULCountForTest(0)

	tac, err := (&eps.TrackingAreaUpdateComplete{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(tac, eps.SHTIntegrityProtectedCiphered, 0, nascommon.DirectionUplink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	if !ue.Conn().SecureExchangeEstablished() {
		t.Fatal("a verified message must establish secure exchange on the connection (TS 24.301 §4.4.4.3)")
	}
}
