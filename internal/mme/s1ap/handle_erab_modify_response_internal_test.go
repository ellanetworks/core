// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestHandoverTriggeredCauses(t *testing.T) {
	radio := func(v int) s1ap.Cause { return s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: v} }

	for cause, want := range map[s1ap.Cause]bool{
		radio(s1ap.CauseRadioNetworkS1IntraSystemHandoverTriggered): true,
		radio(s1ap.CauseRadioNetworkS1InterSystemHandoverTriggered): true,
		radio(s1ap.CauseRadioNetworkX2HandoverTriggered):            true,
		radio(s1ap.CauseRadioNetworkRadioResourcesNotAvailable):     false,
		{Group: s1ap.CauseGroupTransport, Value: 35}:                false,
	} {
		if got := handoverTriggered(cause); got != want {
			t.Errorf("handoverTriggered(%+v) = %v, want %v (TS 36.413 §8.2.2.2)", cause, got, want)
		}
	}
}
