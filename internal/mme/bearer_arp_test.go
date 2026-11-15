// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func TestBearerARP(t *testing.T) {
	for _, tc := range []struct {
		arp  models.Arp
		want s1ap.AllocationAndRetentionPriority
	}{
		{
			models.Arp{PriorityLevel: 9},
			s1ap.AllocationAndRetentionPriority{PriorityLevel: 9, PreemptionCapability: s1ap.PreemptionShallNotTrigger, PreemptionVulnerability: s1ap.PreemptionNotPreemptable},
		},
		{
			models.Arp{PriorityLevel: 9, PreemptCap: models.PreemptionCapabilityNotPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable},
			s1ap.AllocationAndRetentionPriority{PriorityLevel: 9, PreemptionCapability: s1ap.PreemptionShallNotTrigger, PreemptionVulnerability: s1ap.PreemptionNotPreemptable},
		},
		{
			models.Arp{PriorityLevel: 3, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable},
			s1ap.AllocationAndRetentionPriority{PriorityLevel: 3, PreemptionCapability: s1ap.PreemptionMayTrigger, PreemptionVulnerability: s1ap.PreemptionNotPreemptable},
		},
		{
			models.Arp{PriorityLevel: 12, PreemptCap: models.PreemptionCapabilityNotPreempt, PreemptVuln: models.PreemptionVulnerabilityPreemptable},
			s1ap.AllocationAndRetentionPriority{PriorityLevel: 12, PreemptionCapability: s1ap.PreemptionShallNotTrigger, PreemptionVulnerability: s1ap.PreemptionPreemptable},
		},
	} {
		if got := BearerARP(tc.arp); got != tc.want {
			t.Errorf("BearerARP(%+v) = %+v, want %+v", tc.arp, got, tc.want)
		}
	}
}
