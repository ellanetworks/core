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
		{"plain whitelisted", eps.MsgAttachRequest, plain, false, VerdictPlainAllowed},
		{"plain whitelisted detach", eps.MsgDetachRequest, plain, false, VerdictPlainAllowed},
		{"plain non-whitelisted (attach complete)", eps.MsgAttachComplete, plain, false, verdictReject},
		{"plain non-whitelisted (security mode complete)", eps.MsgSecurityModeComplete, plain, false, verdictReject},
		{"protected verified", eps.MsgAttachComplete, prot, true, verdictIntegrityVerified},
		{"protected mac-failed whitelisted", eps.MsgAttachRequest, prot, false, VerdictMacFailedAllowed},
		{"protected mac-failed non-whitelisted", eps.MsgSecurityModeComplete, prot, false, verdictReject},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyNasPdu(tc.mt, tc.sht, tc.macVerified); got != tc.want {
				t.Fatalf("ClassifyNasPdu(%s) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}
