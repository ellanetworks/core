// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §5.5.1.2.5, §5.5.3.2.5
func TestAuthRejectCause(t *testing.T) {
	unknown := fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound)

	tests := []struct {
		name string
		err  error
		want eps.EMMCause
	}{
		{"unknown subscriber", unknown, eps.EMMCauseIMSIUnknownInHSS},
		{"unknown subscriber wrapped", fmt.Errorf("generate EPS vector: %w", unknown), eps.EMMCauseIMSIUnknownInHSS},
		{"raft commit timeout", fmt.Errorf("advance sqn: %w", db.ErrProposeTimeout), eps.EMMCauseNetworkFailure},
		{"forwarded outcome unknown", fmt.Errorf("advance sqn: %w", db.ErrOutcomeUnknown), eps.EMMCauseNetworkFailure},
		{"migration pending", fmt.Errorf("advance sqn: %w", db.ErrMigrationPending), eps.EMMCauseNetworkFailure},
		{"context cancelled", context.Canceled, eps.EMMCauseNetworkFailure},
		{"opaque error defaults to network failure", errors.New("boom"), eps.EMMCauseNetworkFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := authRejectCause(tt.err); got != tt.want {
				t.Errorf("authRejectCause(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
