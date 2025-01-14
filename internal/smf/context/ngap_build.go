// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/omec-project/aper"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
	"github.com/omec-project/openapi/models"
)

const DefaultNonGBR5QI = 9

func BuildPDUSessionResourceSetupRequestTransfer(ctx *SMContext) ([]byte, error) {
	ANUPF := ctx.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode
	UpNode := ANUPF.UPF
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, ANUPF.UpLinkTunnel.TEID)

	resourceSetupRequestTransfer := ngapType.PDUSessionResourceSetupRequestTransfer{}

	// PDU Session Aggregate Maximum Bit Rate
	// This IE is Conditional and shall be present when at least one NonGBR QoS flow is being setup.
	ie := ngapType.PDUSessionResourceSetupRequestTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	sessRule := ctx.SelectedSessionRule()
	if sessRule == nil || sessRule.AuthSessAmbr == nil {
		return nil, fmt.Errorf("no PDU Session AMBR")
	}
	ie.Value = ngapType.PDUSessionResourceSetupRequestTransferIEsValue{
		Present: ngapType.PDUSessionResourceSetupRequestTransferIEsPresentPDUSessionAggregateMaximumBitRate,
		PDUSessionAggregateMaximumBitRate: &ngapType.PDUSessionAggregateMaximumBitRate{
			PDUSessionAggregateMaximumBitRateDL: ngapType.BitRate{
				Value: ngapConvert.UEAmbrToInt64(sessRule.AuthSessAmbr.Downlink),
			},
			PDUSessionAggregateMaximumBitRateUL: ngapType.BitRate{
				Value: ngapConvert.UEAmbrToInt64(sessRule.AuthSessAmbr.Uplink),
			},
		},
	}
	resourceSetupRequestTransfer.ProtocolIEs.List = append(resourceSetupRequestTransfer.ProtocolIEs.List, ie)

	// UL NG-U UP TNL Information
	ie = ngapType.PDUSessionResourceSetupRequestTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDULNGUUPTNLInformation
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	if n3IP, err := UpNode.N3Interfaces[0].IP(ctx.SelectedPDUSessionType); err != nil {
		return nil, err
	} else {
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

	// Get Qos Flows
	var qosAddFlows map[string]*models.QosData

	// Initialise QosFlows with existing Ctxt QosFlows, if any
	if len(ctx.SmPolicyData.SmCtxtQosData.QosData) > 0 {
		qosAddFlows = ctx.SmPolicyData.SmCtxtQosData.QosData
	}

	// PCF has provided some update
	if len(ctx.SmPolicyUpdates) > 0 {
		smPolicyUpdates := ctx.SmPolicyUpdates[0]
		if smPolicyUpdates.QosFlowUpdate != nil && smPolicyUpdates.QosFlowUpdate.GetAddQosFlowUpdate() != nil {
			qosAddFlows = smPolicyUpdates.QosFlowUpdate.GetAddQosFlowUpdate()
		}
	}

	// QoS Flow Setup Request List
	if len(qosAddFlows) > 0 {
		ie = ngapType.PDUSessionResourceSetupRequestTransferIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDQosFlowSetupRequestList
		ie.Criticality.Value = ngapType.CriticalityPresentReject

		var qosFlowsList []ngapType.QosFlowSetupRequestItem
		for _, qosFlow := range qosAddFlows {
			arpPreemptCap := ngapType.PreEmptionCapabilityPresentMayTriggerPreEmption
			if qosFlow.Arp.PreemptCap == models.PreemptionCapability_NOT_PREEMPT {
				arpPreemptCap = ngapType.PreEmptionCapabilityPresentShallNotTriggerPreEmption
			}

			arpPreemptVul := ngapType.PreEmptionVulnerabilityPresentNotPreEmptable
			if qosFlow.Arp.PreemptVuln == models.PreemptionVulnerability_PREEMPTABLE {
				arpPreemptVul = ngapType.PreEmptionVulnerabilityPresentPreEmptable
			}

			qosFlowItem := ngapType.QosFlowSetupRequestItem{
				QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: int64(qos.GetQosFlowIDFromQosID(qosFlow.QosId))},
				QosFlowLevelQosParameters: ngapType.QosFlowLevelQosParameters{
					QosCharacteristics: ngapType.QosCharacteristics{
						Present: ngapType.QosCharacteristicsPresentNonDynamic5QI,
						NonDynamic5QI: &ngapType.NonDynamic5QIDescriptor{
							FiveQI: ngapType.FiveQI{
								Value: int64(qosFlow.Var5qi),
							},
						},
					},
					AllocationAndRetentionPriority: ngapType.AllocationAndRetentionPriority{
						PriorityLevelARP: ngapType.PriorityLevelARP{
							Value: int64(qosFlow.Arp.PriorityLevel),
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
		}

		ie.Value = ngapType.PDUSessionResourceSetupRequestTransferIEsValue{
			Present: ngapType.PDUSessionResourceSetupRequestTransferIEsPresentQosFlowSetupRequestList,
			QosFlowSetupRequestList: &ngapType.QosFlowSetupRequestList{
				List: qosFlowsList,
			},
		}

		resourceSetupRequestTransfer.ProtocolIEs.List = append(resourceSetupRequestTransfer.ProtocolIEs.List, ie)
	}
	/*else {
		//Do not Delete- Might have to enable default Session rule based flow later

		// QoS Flow Setup Request List
		// Get QFI from PCF
		ie = ngapType.PDUSessionResourceSetupRequestTransferIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDQosFlowSetupRequestList
		ie.Criticality.Value = ngapType.CriticalityPresentReject

		arpPreemptCap := ngapType.PreEmptionCapabilityPresentMayTriggerPreEmption
		if sessRule.AuthDefQos.Arp.PreemptCap == models.PreemptionCapability_NOT_PREEMPT {
			arpPreemptCap = ngapType.PreEmptionCapabilityPresentShallNotTriggerPreEmption
		}

		arpPreemptVul := ngapType.PreEmptionVulnerabilityPresentNotPreEmptable
		if sessRule.AuthDefQos.Arp.PreemptVuln == models.PreemptionVulnerability_PREEMPTABLE {
			arpPreemptVul = ngapType.PreEmptionVulnerabilityPresentPreEmptable
		}
		//Default Session Rule
		ie.Value = ngapType.PDUSessionResourceSetupRequestTransferIEsValue{
			Present: ngapType.PDUSessionResourceSetupRequestTransferIEsPresentQosFlowSetupRequestList,
			QosFlowSetupRequestList: &ngapType.QosFlowSetupRequestList{

				List: []ngapType.QosFlowSetupRequestItem{
					{
						QosFlowIdentifier: ngapType.QosFlowIdentifier{
							Value: int64(sessRule.AuthDefQos.Var5qi), //DefaultNonGBR5QI,
						},
						QosFlowLevelQosParameters: ngapType.QosFlowLevelQosParameters{
							QosCharacteristics: ngapType.QosCharacteristics{
								Present: ngapType.QosCharacteristicsPresentNonDynamic5QI,
								NonDynamic5QI: &ngapType.NonDynamic5QIDescriptor{
									FiveQI: ngapType.FiveQI{
										Value: int64(sessRule.AuthDefQos.Var5qi), //DefaultNonGBR5QI,
									},
								},
							},
							AllocationAndRetentionPriority: ngapType.AllocationAndRetentionPriority{
								PriorityLevelARP: ngapType.PriorityLevelARP{
									Value: int64(sessRule.AuthDefQos.Arp.PriorityLevel), //15,
								},
								PreEmptionCapability: ngapType.PreEmptionCapability{
									Value: arpPreemptCap, //ngapType.PreEmptionCapabilityPresentShallNotTriggerPreEmption,
								},
								PreEmptionVulnerability: ngapType.PreEmptionVulnerability{
									Value: arpPreemptVul, //ngapType.PreEmptionVulnerabilityPresentNotPreEmptable,
								},
							},
						},
					},
				},
			},
		}
		resourceSetupRequestTransfer.ProtocolIEs.List = append(resourceSetupRequestTransfer.ProtocolIEs.List, ie)
	}*/

	if buf, err := aper.MarshalWithParams(resourceSetupRequestTransfer, "valueExt"); err != nil {
		return nil, fmt.Errorf("encode resourceSetupRequestTransfer failed: %s", err)
	} else {
		return buf, nil
	}
}

func BuildPDUSessionResourceModifyRequestTransfer(ctx *SMContext) ([]byte, error) {
	resourceModifyRequestTransfer := ngapType.PDUSessionResourceModifyRequestTransfer{}
	ie := ngapType.PDUSessionResourceModifyRequestTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	sessRule := ctx.SelectedSessionRule()
	if sessRule == nil || sessRule.AuthSessAmbr == nil {
		return nil, fmt.Errorf("no PDU Session AMBR")
	}
	ie.Value = ngapType.PDUSessionResourceModifyRequestTransferIEsValue{
		Present: ngapType.PDUSessionResourceModifyRequestTransferIEsPresentPDUSessionAggregateMaximumBitRate,
		PDUSessionAggregateMaximumBitRate: &ngapType.PDUSessionAggregateMaximumBitRate{
			PDUSessionAggregateMaximumBitRateDL: ngapType.BitRate{
				Value: ngapConvert.UEAmbrToInt64(sessRule.AuthSessAmbr.Downlink),
			},
			PDUSessionAggregateMaximumBitRateUL: ngapType.BitRate{
				Value: ngapConvert.UEAmbrToInt64(sessRule.AuthSessAmbr.Uplink),
			},
		},
	}
	resourceModifyRequestTransfer.ProtocolIEs.List = append(resourceModifyRequestTransfer.ProtocolIEs.List, ie)

	// QoS Flow Modify Request List
	// use Default 5qi, arp
	arpPreemptCap := ngapType.PreEmptionCapabilityPresentMayTriggerPreEmption
	if sessRule.AuthDefQos.Arp.PreemptCap == models.PreemptionCapability_NOT_PREEMPT {
		arpPreemptCap = ngapType.PreEmptionCapabilityPresentShallNotTriggerPreEmption
	}

	arpPreemptVul := ngapType.PreEmptionVulnerabilityPresentNotPreEmptable
	if sessRule.AuthDefQos.Arp.PreemptVuln == models.PreemptionVulnerability_PREEMPTABLE {
		arpPreemptVul = ngapType.PreEmptionVulnerabilityPresentPreEmptable
	}

	ie = ngapType.PDUSessionResourceModifyRequestTransferIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDQosFlowAddOrModifyRequestList
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value = ngapType.PDUSessionResourceModifyRequestTransferIEsValue{
		Present: ngapType.PDUSessionResourceModifyRequestTransferIEsPresentQosFlowAddOrModifyRequestList,
		QosFlowAddOrModifyRequestList: &ngapType.QosFlowAddOrModifyRequestList{
			List: []ngapType.QosFlowAddOrModifyRequestItem{
				{
					QosFlowIdentifier: ngapType.QosFlowIdentifier{
						Value: int64(sessRule.AuthDefQos.Var5qi), // DefaultNonGBR5QI,
					},
					QosFlowLevelQosParameters: &ngapType.QosFlowLevelQosParameters{
						QosCharacteristics: ngapType.QosCharacteristics{
							Present: ngapType.QosCharacteristicsPresentNonDynamic5QI,
							NonDynamic5QI: &ngapType.NonDynamic5QIDescriptor{
								FiveQI: ngapType.FiveQI{
									Value: int64(sessRule.AuthDefQos.Var5qi), // DefaultNonGBR5QI,
								},
							},
						},
						AllocationAndRetentionPriority: ngapType.AllocationAndRetentionPriority{
							PriorityLevelARP: ngapType.PriorityLevelARP{
								Value: int64(sessRule.AuthDefQos.Arp.PriorityLevel), // 15,
							},
							PreEmptionCapability: ngapType.PreEmptionCapability{
								Value: arpPreemptCap, // ngapType.PreEmptionCapabilityPresentShallNotTriggerPreEmption,
							},
							PreEmptionVulnerability: ngapType.PreEmptionVulnerability{
								Value: arpPreemptVul, // ngapType.PreEmptionVulnerabilityPresentNotPreEmptable,
							},
						},
					},
				},
			},
		},
	}

	resourceModifyRequestTransfer.ProtocolIEs.List = append(resourceModifyRequestTransfer.ProtocolIEs.List, ie)

	// Encode
	if buf, err := aper.MarshalWithParams(resourceModifyRequestTransfer, "valueExt"); err != nil {
		return nil, fmt.Errorf("encode resourceModifyRequestTransfer failed: %s", err)
	} else {
		return buf, nil
	}
}

func BuildPDUSessionResourceReleaseCommandTransfer(ctx *SMContext) (buf []byte, err error) {
	resourceReleaseCommandTransfer := ngapType.PDUSessionResourceReleaseCommandTransfer{
		Cause: ngapType.Cause{
			Present: ngapType.CausePresentNas,
			Nas: &ngapType.CauseNas{
				Value: ngapType.CauseNasPresentNormalRelease,
			},
		},
	}
	buf, err = aper.MarshalWithParams(resourceReleaseCommandTransfer, "valueExt")
	if err != nil {
		return nil, err
	}
	return
}

// TS 38.413 9.3.4.9
func BuildPathSwitchRequestAcknowledgeTransfer(ctx *SMContext) ([]byte, error) {
	logger.SmfLog.Warnf("BuildPathSwitchRequestAcknowledgeTransfer")
	ANUPF := ctx.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode
	UpNode := ANUPF.UPF
	logger.SmfLog.Warnf("UPF TEID: %v", ANUPF.UpLinkTunnel.TEID)
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, ANUPF.UpLinkTunnel.TEID)

	pathSwitchRequestAcknowledgeTransfer := ngapType.PathSwitchRequestAcknowledgeTransfer{}

	// UL NG-U UP TNL Information(optional) TS 38.413 9.3.2.2
	pathSwitchRequestAcknowledgeTransfer.
		ULNGUUPTNLInformation = new(ngapType.UPTransportLayerInformation)

	ULNGUUPTNLInformation := pathSwitchRequestAcknowledgeTransfer.ULNGUUPTNLInformation
	ULNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	ULNGUUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)

	if n3IP, err := UpNode.N3Interfaces[0].IP(ctx.SelectedPDUSessionType); err != nil {
		return nil, err
	} else {
		gtpTunnel := ULNGUUPTNLInformation.GTPTunnel
		gtpTunnel.GTPTEID.Value = teidOct
		gtpTunnel.TransportLayerAddress.Value = aper.BitString{
			Bytes:     n3IP,
			BitLength: uint64(len(n3IP) * 8),
		}
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

func BuildPathSwitchRequestUnsuccessfulTransfer(causePresent int, causeValue aper.Enumerated) (buf []byte, err error) {
	pathSwitchRequestUnsuccessfulTransfer := ngapType.PathSwitchRequestUnsuccessfulTransfer{}

	pathSwitchRequestUnsuccessfulTransfer.Cause.Present = causePresent
	cause := &pathSwitchRequestUnsuccessfulTransfer.Cause

	switch causePresent {
	case ngapType.CausePresentRadioNetwork:
		cause.RadioNetwork = new(ngapType.CauseRadioNetwork)
		cause.RadioNetwork.Value = causeValue
	case ngapType.CausePresentTransport:
		cause.Transport = new(ngapType.CauseTransport)
		cause.Transport.Value = causeValue
	case ngapType.CausePresentNas:
		cause.Nas = new(ngapType.CauseNas)
		cause.Nas.Value = causeValue
	case ngapType.CausePresentProtocol:
		cause.Protocol = new(ngapType.CauseProtocol)
		cause.Protocol.Value = causeValue
	case ngapType.CausePresentMisc:
		cause.Misc = new(ngapType.CauseMisc)
		cause.Misc.Value = causeValue
	}

	buf, err = aper.MarshalWithParams(pathSwitchRequestUnsuccessfulTransfer, "valueExt")
	if err != nil {
		return nil, err
	}
	return
}

func BuildHandoverCommandTransfer(ctx *SMContext) ([]byte, error) {
	logger.SmfLog.Warnf("BuildHandoverCommandTransfer")
	ANUPF := ctx.Tunnel.DataPathPool.GetDefaultPath().FirstDPNode
	UpNode := ANUPF.UPF
	logger.SmfLog.Warnf("UPF TEID: %v", ANUPF.UpLinkTunnel.TEID)
	teidOct := make([]byte, 4)
	binary.BigEndian.PutUint32(teidOct, ANUPF.UpLinkTunnel.TEID)
	handoverCommandTransfer := ngapType.HandoverCommandTransfer{}

	handoverCommandTransfer.DLForwardingUPTNLInformation = new(ngapType.UPTransportLayerInformation)
	handoverCommandTransfer.DLForwardingUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	handoverCommandTransfer.DLForwardingUPTNLInformation.GTPTunnel = new(ngapType.GTPTunnel)

	if n3IP, err := UpNode.N3Interfaces[0].IP(ctx.SelectedPDUSessionType); err != nil {
		return nil, err
	} else {
		gtpTunnel := handoverCommandTransfer.DLForwardingUPTNLInformation.GTPTunnel
		gtpTunnel.GTPTEID.Value = teidOct
		gtpTunnel.TransportLayerAddress.Value = aper.BitString{
			Bytes:     n3IP,
			BitLength: uint64(len(n3IP) * 8),
		}
	}

	if buf, err := aper.MarshalWithParams(handoverCommandTransfer, "valueExt"); err != nil {
		return nil, err
	} else {
		return buf, nil
	}
}
