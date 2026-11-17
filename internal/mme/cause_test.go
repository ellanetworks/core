// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestS1APCauseName(t *testing.T) {
	tests := []struct {
		name  string
		cause s1ap.Cause
		want  string
	}{
		{"radio user-inactivity", s1ap.CauseRadioNetworkUserInactivity.Cause(), "radioNetwork: user-inactivity (20)"},
		{"radio radio-connection-with-ue-lost", s1ap.CauseRadioNetworkRadioConnectionWithUELost.Cause(), "radioNetwork: radio-connection-with-ue-lost (21)"},
		{"radio unknown-mme-ue-s1ap-id", s1ap.CauseRadioNetworkUnknownMMEUES1APID.Cause(), "radioNetwork: unknown-mme-ue-s1ap-id (13)"},
		{"radio extension n26", s1ap.CauseRadioNetworkN26InterfaceNotAvailable.Cause(), "radioNetwork: n26-interface-not-available (40)"},
		{"nas detach", s1ap.CauseNASDetach.Cause(), "nas: detach (2)"},
		{"misc unknown-PLMN", s1ap.CauseMiscUnknownPLMN.Cause(), "misc: unknown-PLMN (5)"},
		{"radio out of range", s1ap.CauseRadioNetwork(99).Cause(), "radioNetwork: unknown (99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := S1apCauseName(&tt.cause)
			if got != tt.want {
				t.Errorf("S1apCauseName = %q, want %q", got, tt.want)
			}
		})
	}
}
