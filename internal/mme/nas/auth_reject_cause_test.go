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

// TS 24.301 §5.5.1.2.5
func TestAttachRejectCauseForAuthFailure(t *testing.T) {
	unknown := fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound)

	tests := []struct {
		name          string
		err           error
		want          eps.EMMCause
		wantPermanent bool
	}{
		{"unknown subscriber", unknown, eps.EMMCauseIllegalUE, true},
		{"unknown subscriber wrapped", fmt.Errorf("generate EPS vector: %w", unknown), eps.EMMCauseIllegalUE, true},
		{"raft commit timeout", fmt.Errorf("advance sqn: %w", db.ErrProposeTimeout), 0, false},
		{"forwarded outcome unknown", fmt.Errorf("advance sqn: %w", db.ErrOutcomeUnknown), 0, false},
		{"migration pending", fmt.Errorf("advance sqn: %w", db.ErrMigrationPending), 0, false},
		{"context cancelled", context.Canceled, 0, false},
		{"opaque error defaults to transient", errors.New("boom"), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, permanent := attachRejectCauseForAuthFailure(tt.err)

			if permanent != tt.wantPermanent {
				t.Errorf("permanent = %v, want %v", permanent, tt.wantPermanent)
			}

			if got != tt.want {
				t.Errorf("attachRejectCauseForAuthFailure(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestAttachRejectCauseIsAnAttachRejectCause(t *testing.T) {
	listed := map[eps.EMMCause]bool{
		eps.EMMCauseIllegalUE:                                  true,
		eps.EMMCauseIllegalME:                                  true,
		eps.EMMCauseEPSAndNonEPSServicesNotAllowed:             true,
		eps.EMMCauseEPSServicesNotAllowed:                      true,
		eps.EMMCausePLMNNotAllowed:                             true,
		eps.EMMCauseTrackingAreaNotAllowed:                     true,
		eps.EMMCauseRoamingNotAllowedInThisTA:                  true,
		eps.EMMCauseEPSServicesNotAllowedInThisPLMN:            true,
		eps.EMMCauseNoSuitableCellsInTrackingArea:              true,
		eps.EMMCauseCongestion:                                 true,
		eps.EMMCauseNotAuthorizedForThisCSG:                    true,
		eps.EMMCauseRedirectionTo5GCNRequired:                  true,
		eps.EMMCauseServiceOptionNotAuthorizedInPLMN:           true,
		eps.EMMCauseIABNodeOperationNotAuthorized:              true,
		eps.EMMCauseSevereNetworkFailure:                       true,
		eps.EMMCausePLMNNotAllowedToOperateAtPresentUELocation: true,
	}

	for _, err := range []error{
		fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound),
		errors.New("boom"),
		db.ErrProposeTimeout,
	} {
		cause, permanent := attachRejectCauseForAuthFailure(err)
		if !permanent {
			continue
		}

		if !listed[cause] {
			t.Errorf("attachRejectCauseForAuthFailure(%v) = %v, which TS 24.301 §5.5.1.2.5 does not define for an ATTACH REJECT", err, cause)
		}
	}
}
