package nas

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"go.uber.org/zap"
)

type IntegrityProtectionMaximumDataRate struct {
	Uplink   uint8 `json:"uplink"`
	Downlink uint8 `json:"downlink"`
}

type Capability5GSM struct {
	RqoS   uint8 `json:"rqo_s"`
	MH6PDU uint8 `json:"mh_6_pdu"`
}

type ExtendedProtocolConfigurationOptions struct {
	PCSCFIPv6AddressRequestUL                                     *bool `json:"pcscf_ipv6_address_request_ul,omitempty"`
	IMCNSubsystemSignalingFlagUL                                  *bool `json:"imcn_subsystem_signaling_flag_ul,omitempty"`
	DNSServerIPv6AddressRequestUL                                 *bool `json:"dns_server_ipv6_address_request_ul,omitempty"`
	NotSupportedUL                                                *bool `json:"not_supported_ul,omitempty"`
	MSSupportOfNetworkRequestedBearerControlIndicatorUL           *bool `json:"ms_support_of_network_requested_bearer_control_indicator_ul,omitempty"`
	DSMIPv6HomeAgentAddressRequestUL                              *bool `json:"dsm_ipv6_home_agent_address_request_ul,omitempty"`
	DSMIPv6HomeNetworkPrefixRequestUL                             *bool `json:"dsm_ipv6_home_network_prefix_request_ul,omitempty"`
	DSMIPv6IPv4HomeAgentAddressRequestUL                          *bool `json:"dsm_ipv6_ipv4_home_agent_address_request_ul,omitempty"`
	IPAddressAllocationViaNASSignallingUL                         *bool `json:"ip_address_allocation_via_nas_signalling_ul,omitempty"`
	IPv4AddressAllocationViaDHCPv4UL                              *bool `json:"ipv4_address_allocation_via_dhcpv4_ul,omitempty"`
	PCSCFIPv4AddressRequestUL                                     *bool `json:"pcscf_ipv4_address_request_ul,omitempty"`
	DNSServerIPv4AddressRequestUL                                 *bool `json:"dns_server_ipv4_address_request_ul,omitempty"`
	MSISDNRequestUL                                               *bool `json:"msisdn_request_ul,omitempty"`
	IFOMSupportRequestUL                                          *bool `json:"ifom_support_request_ul,omitempty"`
	MSSupportOfLocalAddressInTFTIndicatorUL                       *bool `json:"ms_support_of_local_address_in_tft_indicator_ul,omitempty"`
	PCSCFReSelectionSupportUL                                     *bool `json:"pcscf_re_selection_support_ul,omitempty"`
	NBIFOMRequestIndicatorUL                                      *bool `json:"nbifom_request_indicator_ul,omitempty"`
	NBIFOMModeUL                                                  *bool `json:"nbifom_mode_ul,omitempty"`
	NonIPLinkMTURequestUL                                         *bool `json:"non_ip_link_mtu_request_ul,omitempty"`
	APNRateControlSupportIndicatorUL                              *bool `json:"apn_rate_control_support_indicator_ul,omitempty"`
	UEStatus3GPPPSDataOffUL                                       *bool `json:"ue_status_3gpp_ps_data_off_ul,omitempty"`
	ReliableDataServiceRequestIndicatorUL                         *bool `json:"reliable_data_service_request_indicator_ul,omitempty"`
	AdditionalAPNRateControlForExceptionDataSupportIndicatorUL    *bool `json:"additional_apn_rate_control_for_exception_data_support_indicator_ul,omitempty"`
	PDUSessionIDUL                                                *bool `json:"pdu_session_id_ul,omitempty"`
	EthernetFramePayloadMTURequestUL                              *bool `json:"ethernet_frame_payload_mtu_request_ul,omitempty"`
	UnstructuredLinkMTURequestUL                                  *bool `json:"unstructured_link_mtu_request_ul,omitempty"`
	I5GSMCauseValueUL                                             *bool `json:"i5gsm_cause_value_ul,omitempty"`
	QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL            *bool `json:"qos_rules_with_the_length_of_two_octets_support_indicator_ul,omitempty"`
	QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL *bool `json:"qos_flow_descriptions_with_the_length_of_two_octets_support_indicator_ul,omitempty"`
	LinkControlProtocolUL                                         *bool `json:"link_control_protocol_ul,omitempty"`
	PushAccessControlProtocolUL                                   *bool `json:"push_access_control_protocol_ul,omitempty"`
	ChallengeHandshakeAuthenticationProtocolUL                    *bool `json:"challenge_handshake_authentication_protocol_ul,omitempty"`
	InternetProtocolControlProtocolUL                             *bool `json:"internet_protocol_control_protocol_ul,omitempty"`
}

type PDUSessionEstablishmentRequest struct {
	ExtendedProtocolDiscriminator                 uint8                                 `json:"extended_protocol_discriminator"`
	PDUSessionID                                  uint8                                 `json:"pdu_session_id"`
	PTI                                           uint8                                 `json:"pti"`
	PDUSESSIONESTABLISHMENTREQUESTMessageIdentity uint8                                 `json:"pdu_session_establishment_request_message_identity"`
	IntegrityProtectionMaximumDataRate            IntegrityProtectionMaximumDataRate    `json:"integrity_protection_maximum_data_rate"`
	PDUSessionType                                *string                               `json:"pdu_session_type,omitempty"`
	SSCMode                                       *uint8                                `json:"ssc_mode,omitempty"`
	Capability5GSM                                *Capability5GSM                       `json:"capability_5g_s_m,omitempty"`
	ExtendedProtocolConfigurationOptions          *ExtendedProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
}

func buildPDUSessionEstablishmentRequest(msg *nasMessage.PDUSessionEstablishmentRequest) *PDUSessionEstablishmentRequest {
	if msg == nil {
		return nil
	}

	estReq := &PDUSessionEstablishmentRequest{
		ExtendedProtocolDiscriminator: msg.ExtendedProtocolDiscriminator.Octet,
		PDUSessionID:                  msg.PDUSessionID.GetPDUSessionID(),
		PTI:                           msg.PTI.GetPTI(),
		PDUSESSIONESTABLISHMENTREQUESTMessageIdentity: msg.PDUSESSIONESTABLISHMENTREQUESTMessageIdentity.GetMessageType(),
		IntegrityProtectionMaximumDataRate: IntegrityProtectionMaximumDataRate{
			Uplink:   msg.IntegrityProtectionMaximumDataRate.GetMaximumDataRatePerUEForUserPlaneIntegrityProtectionForUpLink(),
			Downlink: msg.IntegrityProtectionMaximumDataRate.GetMaximumDataRatePerUEForUserPlaneIntegrityProtectionForDownLink(),
		},
	}

	if msg.PDUSessionType != nil {
		sessionType := buildPDUSessionType(msg.PDUSessionType.GetPDUSessionTypeValue())
		estReq.PDUSessionType = &sessionType
	}

	if msg.SSCMode != nil {
		sscMode := msg.SSCMode.GetSSCMode()
		estReq.SSCMode = &sscMode
	}

	if msg.Capability5GSM != nil {
		estReq.Capability5GSM = buildCapability5GSM(*msg.Capability5GSM)
	}

	if msg.MaximumNumberOfSupportedPacketFilters != nil {
		logger.EllaLog.Warn("MaximumNumberOfSupportedPacketFilters not yet implemented")
	}

	if msg.AlwaysonPDUSessionRequested != nil {
		logger.EllaLog.Warn("AlwaysonPDUSessionRequested not yet implemented")
	}

	if msg.SMPDUDNRequestContainer != nil {
		logger.EllaLog.Warn("SMPDUDNRequestContainer not yet implemented")
	}

	if msg.ExtendedProtocolConfigurationOptions != nil {
		estReq.ExtendedProtocolConfigurationOptions = buildExtendedProtocolConfigurationOptions(msg.ExtendedProtocolConfigurationOptions)
	}

	return estReq
}

func buildCapability5GSM(msg nasType.Capability5GSM) *Capability5GSM {
	return &Capability5GSM{
		RqoS:   msg.GetRqoS(),
		MH6PDU: msg.GetMH6PDU(),
	}
}

func ptr(b bool) *bool { return &b }

func buildExtendedProtocolConfigurationOptions(opts *nasType.ExtendedProtocolConfigurationOptions) *ExtendedProtocolConfigurationOptions {
	content := opts.GetExtendedProtocolConfigurationOptionsContents()

	pco := nasConvert.NewProtocolConfigurationOptions()

	unmarshalErr := pco.UnMarshal(content)
	if unmarshalErr != nil {
		logger.EllaLog.Warn("failed to parse extended protocol configuration options content", zap.Error(unmarshalErr))
		return nil
	}

	extOpts := &ExtendedProtocolConfigurationOptions{}

	for _, container := range pco.ProtocolOrContainerList {
		switch container.ProtocolOrContainerID {
		case nasMessage.PCSCFIPv6AddressRequestUL:
			extOpts.PCSCFIPv6AddressRequestUL = ptr(true)
		case nasMessage.IMCNSubsystemSignalingFlagUL:
			extOpts.IMCNSubsystemSignalingFlagUL = ptr(true)
		case nasMessage.DNSServerIPv6AddressRequestUL:
			extOpts.DNSServerIPv6AddressRequestUL = ptr(true)
		case nasMessage.NotSupportedUL:
			extOpts.NotSupportedUL = ptr(true)
		case nasMessage.MSSupportOfNetworkRequestedBearerControlIndicatorUL:
			extOpts.MSSupportOfNetworkRequestedBearerControlIndicatorUL = ptr(true)
		case nasMessage.DSMIPv6HomeAgentAddressRequestUL:
			extOpts.DSMIPv6HomeAgentAddressRequestUL = ptr(true)
		case nasMessage.DSMIPv6HomeNetworkPrefixRequestUL:
			extOpts.DSMIPv6HomeNetworkPrefixRequestUL = ptr(true)
		case nasMessage.DSMIPv6IPv4HomeAgentAddressRequestUL:
			extOpts.DSMIPv6IPv4HomeAgentAddressRequestUL = ptr(true)
		case nasMessage.IPAddressAllocationViaNASSignallingUL:
			extOpts.IPAddressAllocationViaNASSignallingUL = ptr(true)
		case nasMessage.IPv4AddressAllocationViaDHCPv4UL:
			extOpts.IPv4AddressAllocationViaDHCPv4UL = ptr(true)
		case nasMessage.PCSCFIPv4AddressRequestUL:
			extOpts.PCSCFIPv4AddressRequestUL = ptr(true)
		case nasMessage.DNSServerIPv4AddressRequestUL:
			extOpts.DNSServerIPv4AddressRequestUL = ptr(true)
		case nasMessage.MSISDNRequestUL:
			extOpts.MSISDNRequestUL = ptr(true)
		case nasMessage.IFOMSupportRequestUL:
			extOpts.IFOMSupportRequestUL = ptr(true)
		case nasMessage.MSSupportOfLocalAddressInTFTIndicatorUL:
			extOpts.MSSupportOfLocalAddressInTFTIndicatorUL = ptr(true)
		case nasMessage.PCSCFReSelectionSupportUL:
			extOpts.PCSCFReSelectionSupportUL = ptr(true)
		case nasMessage.NBIFOMRequestIndicatorUL:
			extOpts.NBIFOMRequestIndicatorUL = ptr(true)
		case nasMessage.NBIFOMModeUL:
			extOpts.NBIFOMModeUL = ptr(true)
		case nasMessage.NonIPLinkMTURequestUL:
			extOpts.NonIPLinkMTURequestUL = ptr(true)
		case nasMessage.APNRateControlSupportIndicatorUL:
			extOpts.APNRateControlSupportIndicatorUL = ptr(true)
		case nasMessage.UEStatus3GPPPSDataOffUL:
			extOpts.UEStatus3GPPPSDataOffUL = ptr(true)
		case nasMessage.ReliableDataServiceRequestIndicatorUL:
			extOpts.ReliableDataServiceRequestIndicatorUL = ptr(true)
		case nasMessage.AdditionalAPNRateControlForExceptionDataSupportIndicatorUL:
			extOpts.AdditionalAPNRateControlForExceptionDataSupportIndicatorUL = ptr(true)
		case nasMessage.PDUSessionIDUL:
			extOpts.PDUSessionIDUL = ptr(true)
		case nasMessage.EthernetFramePayloadMTURequestUL:
			extOpts.EthernetFramePayloadMTURequestUL = ptr(true)
		case nasMessage.UnstructuredLinkMTURequestUL:
			extOpts.UnstructuredLinkMTURequestUL = ptr(true)
		case nasMessage.I5GSMCauseValueUL:
			extOpts.I5GSMCauseValueUL = ptr(true)
		case nasMessage.QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL:
			extOpts.QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL = ptr(true)
		case nasMessage.QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL:
			extOpts.QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL = ptr(true)
		case nasMessage.LinkControlProtocolUL:
			extOpts.LinkControlProtocolUL = ptr(true)
		case nasMessage.PushAccessControlProtocolUL:
			extOpts.PushAccessControlProtocolUL = ptr(true)
		case nasMessage.ChallengeHandshakeAuthenticationProtocolUL:
			extOpts.ChallengeHandshakeAuthenticationProtocolUL = ptr(true)
		case nasMessage.InternetProtocolControlProtocolUL:
			extOpts.InternetProtocolControlProtocolUL = ptr(true)
		default:
			logger.EllaLog.Warn("Unknown Container ID", zap.Uint16("ContainerID", container.ProtocolOrContainerID))
		}
	}

	return extOpts
}
