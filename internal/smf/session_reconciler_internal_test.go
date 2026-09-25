// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"errors"
	"fmt"
	"testing"
)

func TestPermanentPolicyFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"no matching policy", ErrNoPolicyMatch, true},
		{"data network gone", ErrDNNNotFound, true},
		{"data network unbound from slice", ErrDNNNotInSlice, true},
		{"wrapped", fmt.Errorf("get session policy: %w", ErrDNNNotInSlice), true},
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
