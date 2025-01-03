package context

import (
	nasMessage "github.com/ellanetworks/core/internal/util/nas/message"
	"github.com/ellanetworks/core/internal/util/nas/nasConvert"
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
			smContext.SubCtxLog.Errorf("%s", err)
			return
		}
	} else {
		// Set to default supported PDU Session Type
		switch SMF_Self().SupportedPDUSessionType {
		case "IPv4":
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv4
		case "IPv6":
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv6
		case "IPv4v6":
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv4IPv6
		case "Ethernet":
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeEthernet
		default:
			smContext.SelectedPDUSessionType = nasMessage.PDUSessionTypeIPv4
		}
	}

	if req.ExtendedProtocolConfigurationOptions != nil {
		EPCOContents := req.ExtendedProtocolConfigurationOptions.GetExtendedProtocolConfigurationOptionsContents()
		protocolConfigurationOptions := nasConvert.NewProtocolConfigurationOptions()
		unmarshalErr := protocolConfigurationOptions.UnMarshal(EPCOContents)
		if unmarshalErr != nil {
			smContext.SubGsmLog.Errorf("Parsing PCO failed: %s", unmarshalErr)
		}

		// Send MTU to UE always even if UE does not request it.
		// Preconfiguring MTU request flag.
		smContext.ProtocolConfigurationOptions.IPv4LinkMTURequest = true

		for _, container := range protocolConfigurationOptions.ProtocolOrContainerList {
			smContext.SubGsmLog.Debugln("Container ID: ", container.ProtocolOrContainerID)
			smContext.SubGsmLog.Debugln("Container Length: ", container.LengthOfContents)
			switch container.ProtocolOrContainerID {
			case nasMessage.PCSCFIPv6AddressRequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type PCSCFIPv6AddressRequestUL")
			case nasMessage.IMCNSubsystemSignalingFlagUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type IMCNSubsystemSignalingFlagUL")
			case nasMessage.DNSServerIPv6AddressRequestUL:
				smContext.ProtocolConfigurationOptions.DNSIPv6Request = true
			case nasMessage.NotSupportedUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type NotSupportedUL")
			case nasMessage.MSSupportOfNetworkRequestedBearerControlIndicatorUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type MSSupportOfNetworkRequestedBearerControlIndicatorUL")
			case nasMessage.DSMIPv6HomeAgentAddressRequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type DSMIPv6HomeAgentAddressRequestUL")
			case nasMessage.DSMIPv6HomeNetworkPrefixRequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type DSMIPv6HomeNetworkPrefixRequestUL")
			case nasMessage.DSMIPv6IPv4HomeAgentAddressRequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type DSMIPv6IPv4HomeAgentAddressRequestUL")
			case nasMessage.IPAddressAllocationViaNASSignallingUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type IPAddressAllocationViaNASSignallingUL")
			case nasMessage.IPv4AddressAllocationViaDHCPv4UL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type IPv4AddressAllocationViaDHCPv4UL")
			case nasMessage.PCSCFIPv4AddressRequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type PCSCFIPv4AddressRequestUL")
			case nasMessage.DNSServerIPv4AddressRequestUL:
				smContext.ProtocolConfigurationOptions.DNSIPv4Request = true
			case nasMessage.MSISDNRequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type MSISDNRequestUL")
			case nasMessage.IFOMSupportRequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type IFOMSupportRequestUL")
			case nasMessage.MSSupportOfLocalAddressInTFTIndicatorUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type MSSupportOfLocalAddressInTFTIndicatorUL")
			case nasMessage.PCSCFReSelectionSupportUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type PCSCFReSelectionSupportUL")
			case nasMessage.NBIFOMRequestIndicatorUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type NBIFOMRequestIndicatorUL")
			case nasMessage.NBIFOMModeUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type NBIFOMModeUL")
			case nasMessage.NonIPLinkMTURequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type NonIPLinkMTURequestUL")
			case nasMessage.APNRateControlSupportIndicatorUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type APNRateControlSupportIndicatorUL")
			case nasMessage.UEStatus3GPPPSDataOffUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type UEStatus3GPPPSDataOffUL")
			case nasMessage.ReliableDataServiceRequestIndicatorUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type ReliableDataServiceRequestIndicatorUL")
			case nasMessage.AdditionalAPNRateControlForExceptionDataSupportIndicatorUL:
				smContext.SubGsmLog.Infoln(
					"Didn't Implement container type AdditionalAPNRateControlForExceptionDataSupportIndicatorUL",
				)
			case nasMessage.PDUSessionIDUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type PDUSessionIDUL")
			case nasMessage.EthernetFramePayloadMTURequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type EthernetFramePayloadMTURequestUL")
			case nasMessage.UnstructuredLinkMTURequestUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type UnstructuredLinkMTURequestUL")
			case nasMessage.I5GSMCauseValueUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type 5GSMCauseValueUL")
			case nasMessage.QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL")
			case nasMessage.QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL:
				smContext.SubGsmLog.Infoln(
					"Didn't Implement container type QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL",
				)
			case nasMessage.LinkControlProtocolUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type LinkControlProtocolUL")
			case nasMessage.PushAccessControlProtocolUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type PushAccessControlProtocolUL")
			case nasMessage.ChallengeHandshakeAuthenticationProtocolUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type ChallengeHandshakeAuthenticationProtocolUL")
			case nasMessage.InternetProtocolControlProtocolUL:
				smContext.SubGsmLog.Infoln("Didn't Implement container type InternetProtocolControlProtocolUL")
			default:
				smContext.SubGsmLog.Infof("Unknown Container ID [%d]", container.ProtocolOrContainerID)
			}
		}
	}
}

func (smContext *SMContext) HandlePDUSessionReleaseRequest(req *nasMessage.PDUSessionReleaseRequest) {
	smContext.SubGsmLog.Infof("Handle Pdu Session Release Request")
	smContext.Pti = req.GetPTI()
	err := smContext.ReleaseUeIpAddr()
	if err != nil {
		smContext.SubGsmLog.Errorf("Error in releasing UE IP Addr: %s", err)
	}
}
