// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// TestPlainNasAllowed is the sole authority on the EMM pre-secure-exchange whitelist: a
// message type is admissible without a verified MAC (sent plain, or received
// integrity-protected with a failed MAC — TS 24.301 §4.4.4.3) iff it is on the list.
func TestPlainNasAllowed(t *testing.T) {
	cases := []struct {
		name string
		mt   eps.MessageType
		want bool
	}{
		{"attach request", eps.MsgAttachRequest, true},
		{"detach request", eps.MsgDetachRequest, true},
		{"detach accept", eps.MsgDetachAccept, true},
		{"identity response", eps.MsgIdentityResponse, true},
		{"authentication response", eps.MsgAuthenticationResponse, true},
		{"authentication failure", eps.MsgAuthenticationFailure, true},
		{"security mode reject", eps.MsgSecurityModeReject, true},
		{"attach complete", eps.MsgAttachComplete, false},
		{"security mode complete", eps.MsgSecurityModeComplete, false},
		// TRACKING AREA UPDATE REQUEST is verified at the S1AP resume layer (like SERVICE
		// REQUEST, which is a security-header type, not an EMM message type), so it is
		// deliberately not admitted unverified on the EMM dispatch path (TS 24.301
		// §4.4.4.3); its handler assumes a verified message.
		{"tracking area update request", eps.MsgTrackingAreaUpdateRequest, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := plainNasAllowed(tc.mt); got != tc.want {
				t.Fatalf("plainNasAllowed(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
