// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func (smContext *SMContext) HandlePDUSessionEstablishmentRequest(req *nasMessage.PDUSessionEstablishmentRequest) {
	smContext.PDUSessionID = int32(req.PDUSessionID.GetPDUSessionID())

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
			case nasMessage.IMCNSubsystemSignalingFlagUL:
			case nasMessage.DNSServerIPv6AddressRequestUL:
				smContext.ProtocolConfigurationOptions.DNSIPv6Request = true
			case nasMessage.NotSupportedUL:
			case nasMessage.MSSupportOfNetworkRequestedBearerControlIndicatorUL:
			case nasMessage.DSMIPv6HomeAgentAddressRequestUL:
			case nasMessage.DSMIPv6HomeNetworkPrefixRequestUL:
			case nasMessage.DSMIPv6IPv4HomeAgentAddressRequestUL:
			case nasMessage.IPAddressAllocationViaNASSignallingUL:
			case nasMessage.IPv4AddressAllocationViaDHCPv4UL:
			case nasMessage.PCSCFIPv4AddressRequestUL:
			case nasMessage.DNSServerIPv4AddressRequestUL:
				smContext.ProtocolConfigurationOptions.DNSIPv4Request = true
			case nasMessage.MSISDNRequestUL:
			case nasMessage.IFOMSupportRequestUL:
			case nasMessage.MSSupportOfLocalAddressInTFTIndicatorUL:
			case nasMessage.PCSCFReSelectionSupportUL:
			case nasMessage.NBIFOMRequestIndicatorUL:
			case nasMessage.NBIFOMModeUL:
			case nasMessage.NonIPLinkMTURequestUL:
			case nasMessage.APNRateControlSupportIndicatorUL:
			case nasMessage.UEStatus3GPPPSDataOffUL:
			case nasMessage.ReliableDataServiceRequestIndicatorUL:
			case nasMessage.AdditionalAPNRateControlForExceptionDataSupportIndicatorUL:
			case nasMessage.PDUSessionIDUL:
			case nasMessage.EthernetFramePayloadMTURequestUL:
			case nasMessage.UnstructuredLinkMTURequestUL:
			case nasMessage.I5GSMCauseValueUL:
			case nasMessage.QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL:
			case nasMessage.QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL:
			case nasMessage.LinkControlProtocolUL:
			case nasMessage.PushAccessControlProtocolUL:
			case nasMessage.ChallengeHandshakeAuthenticationProtocolUL:
			case nasMessage.InternetProtocolControlProtocolUL:
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
