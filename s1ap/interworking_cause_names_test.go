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
		{s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell}, "no-radio-resources-available-in-target-cell"},
		{s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkEncryptionAlgorithmsNotSupported}, "encryption-and-or-integrity-protection-algorithms-not-supported"},
		{s1ap.CauseInsufficientUECapabilities, "insufficient-ue-capabilities"},
		{s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: s1ap.CauseMiscOMIntervention}, "om-intervention"},
	} {
		if got := tc.cause.String(); !strings.Contains(got, tc.want) {
			t.Errorf("cause %+v = %q, want it to name %q", tc.cause, got, tc.want)
		}
	}
}
