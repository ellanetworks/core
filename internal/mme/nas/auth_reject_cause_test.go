// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas/eps"
)

func TestAuthRejectCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want eps.EMMCause
	}{
		{
			name: "propose timeout",
			err:  fmt.Errorf("couldn't update subscriber 001010000000001: %w", db.ErrProposeTimeout),
			want: eps.EMMCauseNetworkFailure,
		},
		{
			name: "migration pending",
			err:  fmt.Errorf("SQN advance failed: %w", db.ErrMigrationPending),
			want: eps.EMMCauseNetworkFailure,
		},
		{
			name: "subscriber missing",
			err:  errors.New("subscriber not found"),
			want: eps.EMMCauseIMSIUnknownInHSS,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := authRejectCause(tc.err); got != tc.want {
				t.Fatalf("authRejectCause(%v): want %d, got %d", tc.err, tc.want, got)
			}
		})
	}
}
