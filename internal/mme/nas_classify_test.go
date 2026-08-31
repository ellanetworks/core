// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §4.4.4.3
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
