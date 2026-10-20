// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestReleaseReasonsMapToARecordClosingCause(t *testing.T) {
	want := map[models.SessionReleaseReason]models.RecordClosingCause{
		models.ReleaseUERequested:           models.RecordClosingNormalRelease,
		models.ReleaseDeregistration:        models.RecordClosingNormalRelease,
		models.ReleaseSubscriptionWithdrawn: models.RecordClosingManagementIntervention,
		models.ReleaseReactivationRequested: models.RecordClosingManagementIntervention,
		models.ReleaseStatusMismatch:        models.RecordClosingNormalRelease,
		models.ReleaseDuplicateSessionID:    models.RecordClosingNormalRelease,
		models.ReleaseHandoverFailure:       models.RecordClosingAbnormalRelease,
		models.ReleaseUENotResponding:       models.RecordClosingAbnormalRelease,
		models.ReleaseProtocolError:         models.RecordClosingAbnormalRelease,
		models.ReleaseEstablishmentFailure:  models.RecordClosingAbnormalRelease,
		models.ReleaseAccessReleased:        models.RecordClosingAbnormalRelease,
	}

	if len(want) != len(models.SessionReleaseReasons) {
		t.Fatalf("%d reasons listed, %d classified", len(models.SessionReleaseReasons), len(want))
	}

	for _, reason := range models.SessionReleaseReasons {
		closing, ok := want[reason]
		if !ok {
			t.Fatalf("reason %s is not classified", reason)
		}

		if got := reason.RecordClosing(); got != closing {
			t.Errorf("%s closes the record with %d, want %d", reason, got, closing)
		}
	}
}
