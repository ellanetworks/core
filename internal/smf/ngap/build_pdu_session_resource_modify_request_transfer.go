// Copyright 2026 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

// BuildPDUSessionResourceModifyRequestTransfer constructs the N2 SM Information
// for a PDU Session Resource Modify Request (TS 38.413 §9.3.4.3).
//
// It includes:
//   - PDU Session Aggregate Maximum Bit Rate (when ambr is non-nil)
//   - QoS Flow Add or Modify Request List (when qosData is non-nil)
//
// This is used during network-requested PDU Session Modification (TS 23.502
// §4.3.3.2) to update the gNB with new QoS parameters and/or session AMBR.
func BuildPDUSessionResourceModifyRequestTransfer(ambr *models.Ambr, qosData *models.QosData) ([]byte, error) {
	transfer := ngapType.PDUSessionResourceModifyRequestTransfer{}

	// PDU Session Aggregate Maximum Bit Rate (IE ID 130)
	if ambr != nil {
		ie := ngapType.PDUSessionResourceModifyRequestTransferIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value = ngapType.PDUSessionResourceModifyRequestTransferIEsValue{
			Present: ngapType.PDUSessionResourceModifyRequestTransferIEsPresentPDUSessionAggregateMaximumBitRate,
			PDUSessionAggregateMaximumBitRate: &ngapType.PDUSessionAggregateMaximumBitRate{
				PDUSessionAggregateMaximumBitRateDL: ngapType.BitRate{
					Value: ngapConvert.UEAmbrToInt64(ambr.Downlink),
				},
				PDUSessionAggregateMaximumBitRateUL: ngapType.BitRate{
					Value: ngapConvert.UEAmbrToInt64(ambr.Uplink),
				},
			},
		}
		transfer.ProtocolIEs.List = append(transfer.ProtocolIEs.List, ie)
	}

	// QoS Flow Add or Modify Request List (IE ID 135)
	if qosData != nil {
		arpPreemptCap := ngapType.PreEmptionCapabilityPresentMayTriggerPreEmption
		if qosData.Arp != nil && qosData.Arp.PreemptCap == models.PreemptionCapabilityNotPreempt {
			arpPreemptCap = ngapType.PreEmptionCapabilityPresentShallNotTriggerPreEmption
		}

		arpPreemptVul := ngapType.PreEmptionVulnerabilityPresentNotPreEmptable
		if qosData.Arp != nil && qosData.Arp.PreemptVuln == models.PreemptionVulnerabilityPreemptable {
			arpPreemptVul = ngapType.PreEmptionVulnerabilityPresentPreEmptable
		}

		arpLevel := int64(1)
		if qosData.Arp != nil {
			arpLevel = int64(qosData.Arp.PriorityLevel)
		}

		qosFlowItem := ngapType.QosFlowAddOrModifyRequestItem{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: int64(qosData.QFI)},
			QosFlowLevelQosParameters: &ngapType.QosFlowLevelQosParameters{
				QosCharacteristics: ngapType.QosCharacteristics{
					Present: ngapType.QosCharacteristicsPresentNonDynamic5QI,
					NonDynamic5QI: &ngapType.NonDynamic5QIDescriptor{
						FiveQI: ngapType.FiveQI{
							Value: int64(qosData.Var5qi),
						},
					},
				},
				AllocationAndRetentionPriority: ngapType.AllocationAndRetentionPriority{
					PriorityLevelARP: ngapType.PriorityLevelARP{
						Value: arpLevel,
					},
					PreEmptionCapability: ngapType.PreEmptionCapability{
						Value: arpPreemptCap,
					},
					PreEmptionVulnerability: ngapType.PreEmptionVulnerability{
						Value: arpPreemptVul,
					},
				},
			},
		}

		ie := ngapType.PDUSessionResourceModifyRequestTransferIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDQosFlowAddOrModifyRequestList
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value = ngapType.PDUSessionResourceModifyRequestTransferIEsValue{
			Present: ngapType.PDUSessionResourceModifyRequestTransferIEsPresentQosFlowAddOrModifyRequestList,
			QosFlowAddOrModifyRequestList: &ngapType.QosFlowAddOrModifyRequestList{
				List: []ngapType.QosFlowAddOrModifyRequestItem{qosFlowItem},
			},
		}
		transfer.ProtocolIEs.List = append(transfer.ProtocolIEs.List, ie)
	}

	if len(transfer.ProtocolIEs.List) == 0 {
		return nil, fmt.Errorf("no IEs to encode in PDU Session Resource Modify Request Transfer")
	}

	buf, err := aper.MarshalWithParams(transfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("encode PDU Session Resource Modify Request Transfer: %w", err)
	}

	return buf, nil
}
