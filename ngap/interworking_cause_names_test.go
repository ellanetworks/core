// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"strings"
	"testing"

	"github.com/ellanetworks/core/ngap"
)

func TestInterworkingFailureCauseNames(t *testing.T) {
	for _, tc := range []struct {
		cause ngap.Cause
		want  string
	}{
		{ngap.CauseRadioNetworkNoRadioResourcesInTargetCell.Cause(), "no-radio-resources-available-in-target-cell"},
		{ngap.CauseRadioNetworkEncryptionAlgorithmsNotSupported.Cause(), "encryption-and-or-integrity-protection-algorithms-not-supported"},
		{ngap.CauseRadioNetworkInsufficientUECapabilities.Cause(), "insufficient-ue-capabilities"},
		{ngap.CauseMiscOMIntervention.Cause(), "om-intervention"},
	} {
		if got := tc.cause.String(); !strings.Contains(got, tc.want) {
			t.Errorf("cause %+v = %q, want it to name %q", tc.cause, got, tc.want)
		}
	}
}
