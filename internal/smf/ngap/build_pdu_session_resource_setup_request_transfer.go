package ngap

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

func BuildPDUSessionResourceSetupRequestTransfer(smPolicyData *models.SmPolicyData, teid uint32, n3IP net.IP) ([]byte, error) {
	if smPolicyData == nil {
		return nil, fmt.Errorf("smPolicyData is nil")
	}

	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, teid)

	resourceSetupRequestTransfer := ngapType.PDUSessionResourceSetupRequestTransfer{}

	// PDU Session Aggregate Maximum Bit Rate
	// This IE is Conditional and shall be present when at least one NonGBR QoS flow is being setup.
	ie := ngapType.PDUSessionResourceSetupRequestTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate
	ie.Criticality.Value = ngapType.CriticalityPresentReject

	ie.Value = ngapType.PDUSessionResourceSetupRequestTransferIEsValue{
		Present: ngapType.PDUSessionResourceSetupRequestTransferIEsPresentPDUSessionAggregateMaximumBitRate,
		PDUSessionAggregateMaximumBitRate: &ngapType.PDUSessionAggregateMaximumBitRate{
			PDUSessionAggregateMaximumBitRateDL: ngapType.BitRate{
				Value: ngapConvert.UEAmbrToInt64(smPolicyData.Ambr.Downlink),
			},
			PDUSessionAggregateMaximumBitRateUL: ngapType.BitRate{
				Value: ngapConvert.UEAmbrToInt64(smPolicyData.Ambr.Uplink),
			},
		},
	}
	resourceSetupRequestTransfer.ProtocolIEs.List = append(resourceSetupRequestTransfer.ProtocolIEs.List, ie)

	// UL NG-U UP TNL Information
	ie = ngapType.PDUSessionResourceSetupRequestTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDULNGUUPTNLInformation
	ie.Criticality.Value = ngapType.CriticalityPresentReject

	ie.Value = ngapType.PDUSessionResourceSetupRequestTransferIEsValue{
		Present: ngapType.PDUSessionResourceSetupRequestTransferIEsPresentULNGUUPTNLInformation,
		ULNGUUPTNLInformation: &ngapType.UPTransportLayerInformation{
			Present: ngapType.UPTransportLayerInformationPresentGTPTunnel,
			GTPTunnel: &ngapType.GTPTunnel{
				TransportLayerAddress: ngapType.TransportLayerAddress{
					Value: aper.BitString{
						Bytes:     n3IP,
						BitLength: uint64(len(n3IP) * 8),
					},
				},
				GTPTEID: ngapType.GTPTEID{Value: teidOct},
			},
		},
	}

	resourceSetupRequestTransfer.ProtocolIEs.List = append(resourceSetupRequestTransfer.ProtocolIEs.List, ie)

	// PDU Session Type
	ie = ngapType.PDUSessionResourceSetupRequestTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionType
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value = ngapType.PDUSessionResourceSetupRequestTransferIEsValue{
		Present: ngapType.PDUSessionResourceSetupRequestTransferIEsPresentPDUSessionType,
		PDUSessionType: &ngapType.PDUSessionType{
			Value: ngapType.PDUSessionTypePresentIpv4,
		},
	}
	resourceSetupRequestTransfer.ProtocolIEs.List = append(resourceSetupRequestTransfer.ProtocolIEs.List, ie)

	// QoS Flow Setup Request List
	if smPolicyData.QosData != nil {
		ie = ngapType.PDUSessionResourceSetupRequestTransferIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDQosFlowSetupRequestList
		ie.Criticality.Value = ngapType.CriticalityPresentReject

		var qosFlowsList []ngapType.QosFlowSetupRequestItem

		arpPreemptCap := ngapType.PreEmptionCapabilityPresentMayTriggerPreEmption
		if smPolicyData.QosData.Arp.PreemptCap == models.PreemptionCapabilityNotPreempt {
			arpPreemptCap = ngapType.PreEmptionCapabilityPresentShallNotTriggerPreEmption
		}

		arpPreemptVul := ngapType.PreEmptionVulnerabilityPresentNotPreEmptable
		if smPolicyData.QosData.Arp.PreemptVuln == models.PreemptionVulnerabilityPreemptable {
			arpPreemptVul = ngapType.PreEmptionVulnerabilityPresentPreEmptable
		}

		qosFlowItem := ngapType.QosFlowSetupRequestItem{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: int64(smPolicyData.QosData.QFI)},
			QosFlowLevelQosParameters: ngapType.QosFlowLevelQosParameters{
				QosCharacteristics: ngapType.QosCharacteristics{
					Present: ngapType.QosCharacteristicsPresentNonDynamic5QI,
					NonDynamic5QI: &ngapType.NonDynamic5QIDescriptor{
						FiveQI: ngapType.FiveQI{
							Value: int64(smPolicyData.QosData.Var5qi),
						},
					},
				},
				AllocationAndRetentionPriority: ngapType.AllocationAndRetentionPriority{
					PriorityLevelARP: ngapType.PriorityLevelARP{
						Value: int64(smPolicyData.QosData.Arp.PriorityLevel),
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
		qosFlowsList = append(qosFlowsList, qosFlowItem)

		ie.Value = ngapType.PDUSessionResourceSetupRequestTransferIEsValue{
			Present: ngapType.PDUSessionResourceSetupRequestTransferIEsPresentQosFlowSetupRequestList,
			QosFlowSetupRequestList: &ngapType.QosFlowSetupRequestList{
				List: qosFlowsList,
			},
		}

		resourceSetupRequestTransfer.ProtocolIEs.List = append(resourceSetupRequestTransfer.ProtocolIEs.List, ie)
	}

	buf, err := aper.MarshalWithParams(resourceSetupRequestTransfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("encode resourceSetupRequestTransfer failed: %s", err)
	}

	return buf, nil
}
