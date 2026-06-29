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
		{"radio user-inactivity", s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 20}, "Radio Network: user-inactivity (20)"},
		{"radio radio-connection-with-ue-lost", s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 21}, "Radio Network: radio-connection-with-ue-lost (21)"},
		{"radio unknown-mme-ue-s1ap-id", s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 13}, "Radio Network: unknown-mme-ue-s1ap-id (13)"},
		{"radio extension n26", s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 4, Extended: true}, "Radio Network: n26-interface-not-available (40)"},
		{"nas detach", s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: 2}, "NAS: detach (2)"},
		{"misc unknown-PLMN", s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 5}, "Misc: unknown-PLMN (5)"},
		{"radio out of range", s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 99}, "Radio Network: unknown (99)"},
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
