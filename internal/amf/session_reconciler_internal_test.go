// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/smf"
)

// Every provisioning failure the SMF distinguishes has to be listed: one left
// out is not a missed release but an endless one, the reconciler retrying a
// session whose policy will never resolve.
func TestPermanentPolicyFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"no matching policy", smf.ErrNoPolicyMatch, true},
		{"data network gone", smf.ErrDNNNotFound, true},
		{"data network unbound from slice", smf.ErrDNNNotInSlice, true},
		{"wrapped", fmt.Errorf("get session policy: %w", smf.ErrDNNNotInSlice), true},
		{"transient infrastructure error", errors.New("raft: propose timeout"), false},
		{"nil", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := permanentPolicyFailure(tc.err); got != tc.want {
				t.Errorf("permanentPolicyFailure(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
