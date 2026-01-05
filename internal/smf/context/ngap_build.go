// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

func BuildPDUSessionResourceSetupRequestTransfer(sessionRule *models.SessionRule, qosData *models.QosData, teid uint32, n3IP net.IP) ([]byte, error) {
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
				Value: ngapConvert.UEAmbrToInt64(sessionRule.AuthSessAmbr.Downlink),
			},
			PDUSessionAggregateMaximumBitRateUL: ngapType.BitRate{
				Value: ngapConvert.UEAmbrToInt64(sessionRule.AuthSessAmbr.Uplink),
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

	// Get Qos Flow
	// var qosAddFlow *models.QosData

	// if qosPolicyData != nil && qosPolicyData.QosData != nil {
	// 	qosAddFlow = qosPolicyData.QosData
	// }

	// // PCF has provided some update
	// if smPolicyUpdates != nil && smPolicyUpdates.QosFlowUpdate != nil {
	// 	qosAddFlow = smPolicyUpdates.QosFlowUpdate
	// }

	// QoS Flow Setup Request List
	if qosData != nil {
		ie = ngapType.PDUSessionResourceSetupRequestTransferIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDQosFlowSetupRequestList
		ie.Criticality.Value = ngapType.CriticalityPresentReject

		var qosFlowsList []ngapType.QosFlowSetupRequestItem

		arpPreemptCap := ngapType.PreEmptionCapabilityPresentMayTriggerPreEmption
		if qosData.Arp.PreemptCap == models.PreemptionCapabilityNotPreempt {
			arpPreemptCap = ngapType.PreEmptionCapabilityPresentShallNotTriggerPreEmption
		}

		arpPreemptVul := ngapType.PreEmptionVulnerabilityPresentNotPreEmptable
		if qosData.Arp.PreemptVuln == models.PreemptionVulnerabilityPreemptable {
			arpPreemptVul = ngapType.PreEmptionVulnerabilityPresentPreEmptable
		}

		qosFlowItem := ngapType.QosFlowSetupRequestItem{
			QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: int64(qosData.QFI)},
			QosFlowLevelQosParameters: ngapType.QosFlowLevelQosParameters{
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
						Value: int64(qosData.Arp.PriorityLevel),
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

func BuildPDUSessionResourceReleaseCommandTransfer() ([]byte, error) {
	resourceReleaseCommandTransfer := ngapType.PDUSessionResourceReleaseCommandTransfer{
		Cause: ngapType.Cause{
			Present: ngapType.CausePresentNas,
			Nas: &ngapType.CauseNas{
				Value: ngapType.CauseNasPresentNormalRelease,
			},
		},
	}

	buf, err := aper.MarshalWithParams(resourceReleaseCommandTransfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not encode pdu session resource release command transfer: %s", err)
	}

	return buf, nil
}

// TS 38.413 9.3.4.9
func BuildPathSwitchRequestAcknowledgeTransfer(teid uint32, n3IP net.IP) ([]byte, error) {
	// dataPath := ctx.Tunnel.DataPath
	// ANUPF := dataPath.DPNode
	// UpNode := ANUPF.UPF
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, teid)

	pathSwitchRequestAcknowledgeTransfer := ngapType.PathSwitchRequestAcknowledgeTransfer{}

	// UL NG-U UP TNL Information(optional) TS 38.413 9.3.2.2
	pathSwitchRequestAcknowledgeTransfer.
		ULNGUUPTNLInformation = new(ngapType.UPTransportLayerInformation)

	ULNGUUPTNLInformation := pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation
	ULNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	ULNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)

	gtpTunnel := ULNGUUPTNLInformation.GTPTunnel
	gtpTunnel.GTPTEID.Value = teidOct
	gtpTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     n3IP,
		BitLength: uint64(len(n3IP) * 8),
	}

	// Security Indication(optional) TS 38.413 9.3.1.27
	pathSwitchRequestAcknowledgeTransfer.SecurityIndication = new(ngapType.SecurityIndication)
	securityIndication := pathSwitchRequestAcknowledgeTransfer.SecurityIndication
	securityIndication.IntegrityProtectionIndication.Value = ngapType.IntegrityProtectionIndicationPresentNotNeeded
	securityIndication.ConfidentialityProtectionIndication.Value = ngapType.ConfidentialityProtectionIndicationPresentNotNeeded

	integrityProtectionInd := securityIndication.IntegrityProtectionIndication.Value
	if integrityProtectionInd == ngapType.IntegrityProtectionIndicationPresentRequired ||
		integrityProtectionInd == ngapType.IntegrityProtectionIndicationPresentPreferred {
		securityIndication.MaximumIntegrityProtectedDataRateUL = new(ngapType.MaximumIntegrityProtectedDataRate)
		securityIndication.MaximumIntegrityProtectedDataRateUL.Value = ngapType.MaximumIntegrityProtectedDataRatePresentBitrate64kbs
	}

	if buf, err := aper.MarshalWithParams(pathSwitchRequestAcknowledgeTransfer, "valueExt"); err != nil {
		return nil, err
	} else {
		return buf, nil
	}
}

func BuildHandoverCommandTransfer(teid uint32, n3IP net.IP) ([]byte, error) {
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, teid)

	handoverCommandTransfer := ngapType.HandoverCommandTransfer{}

	handoverCommandTransfer.DLForwardingUPTNLInformation = new(ngapType.UPTransportLayerInformation)
	handoverCommandTransfer.DLForwardingUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	handoverCommandTransfer.DLForwardingUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)

	gtpTunnel := handoverCommandTransfer.DLForwardingUPTNLInformation.GTPTunnel
	gtpTunnel.GTPTEID.Value = teidOct
	gtpTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     n3IP,
		BitLength: uint64(len(n3IP) * 8),
	}

	buf, err := aper.MarshalWithParams(handoverCommandTransfer, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not encode handover command transfer: %s", err)
	}

	return buf, nil
}
