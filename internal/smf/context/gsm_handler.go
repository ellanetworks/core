// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"go.uber.org/zap"
)

func (smContext *SMContext) HandlePDUSessionEstablishmentRequest(req *nasMessage.PDUSessionEstablishmentRequest) {
	// Retrieve PDUSessionID
	smContext.PDUSessionID = int32(req.PDUSessionID.GetPDUSessionID())

	// Retrieve PTI (Procedure transaction identity)
	smContext.Pti = req.GetPTI()

	// Handle PDUSessionType
	if req.PDUSessionType != nil {
		requestedPDUSessionType := req.PDUSessionType.GetPDUSessionTypeValue()
		if err := smContext.isAllowedPDUSessionType(requestedPDUSessionType); err != nil {
			smContext.SubCtxLog.Error("Requested PDUSessionType is not allowed", zap.Error(err))
			return
		}
	} else {
		smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv4
	}

	if req.ExtendedProtocolConfigurationOptions != nil {
		EPCOContents := req.ExtendedProtocolConfigurationOptions.GetExtendedProtocolConfigurationOptionsContents()
		protocolConfigurationOptions := nasConvert.NewProtocolConfigurationOptions()
		unmarshalErr := protocolConfigurationOptions.UnMarshal(EPCOContents)
		if unmarshalErr != nil {
			smContext.SubGsmLog.Error("Parsing PCO failed", zap.Error(unmarshalErr))
		}

		// Send MTU to UE always even if UE does not request it.
		// Preconfiguring MTU request flag.
		smContext.ProtocolConfigurationOptions.IPv4LinkMTURequest = true

		for _, container := range protocolConfigurationOptions.ProtocolOrContainerList {
			switch container.ProtocolOrContainerID {
			case nasMessage.PCSCFIPv6AddressRequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type PCSCFIPv6AddressRequestUL")
			case nasMessage.IMCNSubsystemSignalingFlagUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type IMCNSubsystemSignalingFlagUL")
			case nasMessage.DNSServerIPv6AddressRequestUL:
				smContext.ProtocolConfigurationOptions.DNSIPv6Request = true
			case nasMessage.NotSupportedUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type NotSupportedUL")
			case nasMessage.MSSupportOfNetworkRequestedBearerControlIndicatorUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type MSSupportOfNetworkRequestedBearerControlIndicatorUL")
			case nasMessage.DSMIPv6HomeAgentAddressRequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type DSMIPv6HomeAgentAddressRequestUL")
			case nasMessage.DSMIPv6HomeNetworkPrefixRequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type DSMIPv6HomeNetworkPrefixRequestUL")
			case nasMessage.DSMIPv6IPv4HomeAgentAddressRequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type DSMIPv6IPv4HomeAgentAddressRequestUL")
			case nasMessage.IPAddressAllocationViaNASSignallingUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type IPAddressAllocationViaNASSignallingUL")
			case nasMessage.IPv4AddressAllocationViaDHCPv4UL:
				smContext.SubGsmLog.Debug("Didn't Implement container type IPv4AddressAllocationViaDHCPv4UL")
			case nasMessage.PCSCFIPv4AddressRequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type PCSCFIPv4AddressRequestUL")
			case nasMessage.DNSServerIPv4AddressRequestUL:
				smContext.ProtocolConfigurationOptions.DNSIPv4Request = true
			case nasMessage.MSISDNRequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type MSISDNRequestUL")
			case nasMessage.IFOMSupportRequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type IFOMSupportRequestUL")
			case nasMessage.MSSupportOfLocalAddressInTFTIndicatorUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type MSSupportOfLocalAddressInTFTIndicatorUL")
			case nasMessage.PCSCFReSelectionSupportUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type PCSCFReSelectionSupportUL")
			case nasMessage.NBIFOMRequestIndicatorUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type NBIFOMRequestIndicatorUL")
			case nasMessage.NBIFOMModeUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type NBIFOMModeUL")
			case nasMessage.NonIPLinkMTURequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type NonIPLinkMTURequestUL")
			case nasMessage.APNRateControlSupportIndicatorUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type APNRateControlSupportIndicatorUL")
			case nasMessage.UEStatus3GPPPSDataOffUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type UEStatus3GPPPSDataOffUL")
			case nasMessage.ReliableDataServiceRequestIndicatorUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type ReliableDataServiceRequestIndicatorUL")
			case nasMessage.AdditionalAPNRateControlForExceptionDataSupportIndicatorUL:
				smContext.SubGsmLog.Debug(
					"Didn't Implement container type AdditionalAPNRateControlForExceptionDataSupportIndicatorUL",
				)
			case nasMessage.PDUSessionIDUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type PDUSessionIDUL")
			case nasMessage.EthernetFramePayloadMTURequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type EthernetFramePayloadMTURequestUL")
			case nasMessage.UnstructuredLinkMTURequestUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type UnstructuredLinkMTURequestUL")
			case nasMessage.I5GSMCauseValueUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type 5GSMCauseValueUL")
			case nasMessage.QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL")
			case nasMessage.QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL:
				smContext.SubGsmLog.Debug(
					"Didn't Implement container type QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL",
				)
			case nasMessage.LinkControlProtocolUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type LinkControlProtocolUL")
			case nasMessage.PushAccessControlProtocolUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type PushAccessControlProtocolUL")
			case nasMessage.ChallengeHandshakeAuthenticationProtocolUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type ChallengeHandshakeAuthenticationProtocolUL")
			case nasMessage.InternetProtocolControlProtocolUL:
				smContext.SubGsmLog.Debug("Didn't Implement container type InternetProtocolControlProtocolUL")
			default:
				smContext.SubGsmLog.Info("Unknown Container ID", zap.Uint16("ContainerID", container.ProtocolOrContainerID))
			}
		}
	}
}

func (smContext *SMContext) HandlePDUSessionReleaseRequest(ctx context.Context, req *nasMessage.PDUSessionReleaseRequest) {
	smContext.Pti = req.GetPTI()
	err := smContext.ReleaseUeIPAddr(ctx)
	if err != nil {
		smContext.SubGsmLog.Error("Releasing UE IP Addr", zap.Error(err))
		return
	}
	logger.SmfLog.Info("Successfully completed PDU Session Release Request", zap.Int32("PDUSessionID", smContext.PDUSessionID))
}
