// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/smf"
)

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
