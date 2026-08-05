// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	libngap "github.com/ellanetworks/core/ngap"
)

// qosFlowLevelQosParameters maps a policy QoS rule onto the NGAP IE. TS 38.413
// bounds priorityLevelARP at 1..15, so a policy carrying 0 is rejected here
// rather than encoded: the value has no meaning on the wire.
func qosFlowLevelQosParameters(qosData *models.QosData) (libngap.QosFlowLevelQosParameters, error) {
	preemptCap := libngap.PreemptionMayTrigger
	preemptVuln := libngap.PreemptionNotPreemptable
	priority := uint8(1)

	if qosData.Arp != nil {
		if qosData.Arp.PreemptCap == models.PreemptionCapabilityNotPreempt {
			preemptCap = libngap.PreemptionShallNotTrigger
		}

		if qosData.Arp.PreemptVuln == models.PreemptionVulnerabilityPreemptable {
			preemptVuln = libngap.PreemptionPreemptable
		}

		priority = uint8(qosData.Arp.PriorityLevel)
	}

	if priority < 1 || priority > 15 {
		return libngap.QosFlowLevelQosParameters{}, fmt.Errorf("ARP priority level %d outside 1..15", priority)
	}

	return libngap.QosFlowLevelQosParameters{
		QosCharacteristics: libngap.QosCharacteristics{
			Kind:          libngap.QosCharacteristicsNonDynamic5QI,
			NonDynamic5QI: libngap.NonDynamic5QIDescriptor{FiveQI: libngap.FiveQI(qosData.Var5qi)},
		},
		AllocationAndRetentionPriority: libngap.AllocationAndRetentionPriority{
			PriorityLevelARP:        libngap.PriorityLevelARP(priority),
			PreemptionCapability:    preemptCap,
			PreemptionVulnerability: preemptVuln,
		},
	}, nil
}

// sessionAMBR converts a policy AMBR pair into the NGAP IE.
func sessionAMBR(ambr *models.Ambr) *libngap.PDUSessionAggregateMaximumBitRate {
	return &libngap.PDUSessionAggregateMaximumBitRate{
		DL: libngap.BitRate(ambr.Downlink.Bps()),
		UL: libngap.BitRate(ambr.Uplink.Bps()),
	}
}
