// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestDeregistrationAcceptAndStatusAreProtectedOnceSecureExchangeIsEstablished(t *testing.T) {
	for _, tc := range []struct {
		name string
		send func(*UeConn)
	}{
		{"DEREGISTRATION ACCEPT", func(c *UeConn) { SendDeregistrationAccept(t.Context(), c) }},
		{"5GMM STATUS", func(c *UeConn) { SendStatus5GMM(t.Context(), c, fgs.GMMCauseProtocolErrorUnspecified) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue, sender := newDownlinkOrderUE(t)
			ue.Conn().MarkSecureExchangeEstablished()

			tc.send(ue.Conn())

			_, shts := sender.awaitWrites(t, 1)
			if fgs.SecurityHeaderType(shts[0]) == fgs.SHTPlain {
				t.Fatalf("%s sent plain after secure exchange was established; the UE discards it (TS 24.501 §4.4.4.2)", tc.name)
			}
		})
	}
}
