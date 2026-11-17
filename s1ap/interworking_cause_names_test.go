// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap_test

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestInterworkingFailureCauseNames(t *testing.T) {
	for _, tc := range []struct {
		cause s1ap.Cause
		want  string
	}{
		{s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell.Cause(), "no-radio-resources-available-in-target-cell"},
		{s1ap.CauseRadioNetworkEncryptionAlgorithmsNotSupported.Cause(), "encryption-and-or-integrity-protection-algorithms-not-supported"},
		{s1ap.CauseRadioNetworkInsufficientUECapabilities.Cause(), "insufficient-ue-capabilities"},
		{s1ap.CauseMiscOMIntervention.Cause(), "om-intervention"},
	} {
		if got := tc.cause.String(); !strings.Contains(got, tc.want) {
			t.Errorf("cause %+v = %q, want it to name %q", tc.cause, got, tc.want)
		}
	}
}
