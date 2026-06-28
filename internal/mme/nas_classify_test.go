// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
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
		{"plain non-whitelisted (attach complete)", eps.MsgAttachComplete, plain, false, verdictReject},
		{"plain non-whitelisted (security mode complete)", eps.MsgSecurityModeComplete, plain, false, verdictReject},
		{"protected verified", eps.MsgAttachComplete, prot, true, verdictIntegrityVerified},
		{"protected mac-failed whitelisted", eps.MsgAttachRequest, prot, false, verdictMacFailedAllowed},
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

// TestPlainNonWhitelistedDiscarded asserts handleNAS drops a plain EMM message
// whose type is not on the §4.4.4.3 whitelist, rather than dispatching it.
func TestPlainNonWhitelistedDiscarded(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	// An un-established connection so the plain whitelist (not the secure-exchange
	// discard) is what gates the message.
	ue.s1.secureExchangeEstablished = false

	// A plain ATTACH COMPLETE is not on the whitelist; it must be discarded, so the
	// UE remains EMM-REGISTERED rather than being driven by an unauthenticated msg.
	plain, err := (&eps.AttachComplete{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(context.Background(), ue, plain)

	if ue.emmState.load() != EMMRegistered {
		t.Fatal("a plain non-whitelisted message must be discarded, not processed (TS 24.301 §4.4.4.3)")
	}
}
