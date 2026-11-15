// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestHandoverTriggeredCauses(t *testing.T) {
	for _, tc := range []struct {
		cause s1ap.Cause
		want  bool
	}{
		{s1ap.CauseRadioNetworkS1IntraSystemHandoverTriggered.Cause(), true},
		{s1ap.CauseRadioNetworkS1InterSystemHandoverTriggered.Cause(), true},
		{s1ap.CauseRadioNetworkX2HandoverTriggered.Cause(), true},
		{s1ap.CauseRadioNetworkRadioResourcesNotAvailable.Cause(), false},
		{s1ap.CauseTransport(35).Cause(), false},
	} {
		if got := handoverTriggered(tc.cause); got != tc.want {
			t.Errorf("handoverTriggered(%+v) = %v, want %v (TS 36.413 §8.2.2.2)", tc.cause, got, tc.want)
		}
	}
}
