// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestDefaultQoSCarriesThePolicyARP(t *testing.T) {
	arp := models.Arp{PriorityLevel: 3, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable}

	p := &Policy{}
	applyDecision(p, &PolicyDecision{Var5qi: 69, Arp: arp})

	if p.QosData.Arp == nil || *p.QosData.Arp != arp {
		t.Fatalf("5G default QoS ARP = %+v, want %+v", p.QosData.Arp, arp)
	}

	if got := epsBearerQoS(p).ARP; got != arp {
		t.Fatalf("4G default bearer ARP = %+v, want %+v", got, arp)
	}
}
