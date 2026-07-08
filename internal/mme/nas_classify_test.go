// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func TestClassifyNasPdu(t *testing.T) {
	plain := uint8(eps.SHTPlain)
	prot := uint8(eps.SHTIntegrityProtected)

	cases := []struct {
		name        string
		mt          eps.MessageType
		sht         uint8
		macVerified bool
		want        verdict
	}{
		{"plain whitelisted", eps.MsgAttachRequest, plain, false, verdictPlainAllowed},
		{"plain whitelisted detach", eps.MsgDetachRequest, plain, false, verdictPlainAllowed},
		{"plain whitelisted detach accept", eps.MsgDetachAccept, plain, false, verdictPlainAllowed},
		{"plain non-whitelisted (attach complete)", eps.MsgAttachComplete, plain, false, verdictReject},
		{"plain non-whitelisted (security mode complete)", eps.MsgSecurityModeComplete, plain, false, verdictReject},
		// TRACKING AREA UPDATE REQUEST is verified at the S1AP resume layer, so it is
		// deliberately not admitted unverified on the EMM dispatch path (TS 24.301
		// §4.4.4.3); its handler assumes a verified message.
		{"plain TAU request rejected", eps.MsgTrackingAreaUpdateRequest, plain, false, verdictReject},
		{"protected mac-failed TAU rejected", eps.MsgTrackingAreaUpdateRequest, prot, false, verdictReject},
		{"protected verified", eps.MsgAttachComplete, prot, true, verdictIntegrityVerified},
		{"protected mac-failed whitelisted", eps.MsgAttachRequest, prot, false, verdictMacFailedAllowed},
		{"protected mac-failed detach accept", eps.MsgDetachAccept, prot, false, verdictMacFailedAllowed},
		{"protected mac-failed non-whitelisted", eps.MsgSecurityModeComplete, prot, false, verdictReject},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyNasPdu(tc.mt, tc.sht, tc.macVerified); got != tc.want {
				t.Fatalf("classifyNasPdu(%s) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}
