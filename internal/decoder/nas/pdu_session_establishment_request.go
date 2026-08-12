// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
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

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

type PDUSessionEstablishmentRequest struct {
	IntegrityProtectionMaximumDataRate   IntegrityProtectionMaximumDataRate    `json:"integrity_protection_maximum_data_rate"`
	PDUSessionType                       *utils.EnumField                      `json:"pdu_session_type,omitempty"`
	SSCMode                              *uint8                                `json:"ssc_mode,omitempty"`
	Capability5GSM                       *Capability5GSM                       `json:"capability_5g_s_m,omitempty"`
	ExtendedProtocolConfigurationOptions *ExtendedProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	MaximumNumberOfSupportedPacketFilters *UnsupportedIE `json:"maximum_number_of_supported_packet_filters,omitempty"`
	AlwaysonPDUSessionRequested           *UnsupportedIE `json:"alwayson_pdu_session_requested,omitempty"`
	SMPDUDNRequestContainer               *UnsupportedIE `json:"smpdu_dn_request_container,omitempty"`
}

func buildPDUSessionEstablishmentRequest(msg *fgs.PDUSessionEstablishmentRequest) *PDUSessionEstablishmentRequest {
	out := &PDUSessionEstablishmentRequest{
		IntegrityProtectionMaximumDataRate: IntegrityProtectionMaximumDataRate{
			Uplink:   msg.IntegrityProtMaxDataRate[0],
			Downlink: msg.IntegrityProtMaxDataRate[1],
		},
	}

	if msg.PDUSessionType != nil {
		sessionType := buildPDUSessionType(uint8(*msg.PDUSessionType))
		out.PDUSessionType = &sessionType
	}

	if msg.SSCMode != nil {
		mode := uint8(*msg.SSCMode)
		out.SSCMode = &mode
	}

	if msg.GSMCapability != nil {
		out.Capability5GSM = &Capability5GSM{
			RqoS:   b2u(msg.GSMCapability.RqoS),
			MH6PDU: b2u(msg.GSMCapability.MH6PDU),
		}
	}

	if msg.AlwaysOnRequested != nil {
		out.AlwaysonPDUSessionRequested = makeUnsupportedIE()
	}

	if msg.ExtendedPCO != nil {
		out.ExtendedProtocolConfigurationOptions = extendedPCOFromNAS(*msg.ExtendedPCO)
	}

	for _, ie := range msg.Unrecognized {
		switch ie.IEI {
		case ieiMaxPacketFilters:
			out.MaximumNumberOfSupportedPacketFilters = makeUnsupportedIE()
		case ieiSMPDUDNRequest:
			out.SMPDUDNRequestContainer = makeUnsupportedIE()
		case ieiExtendedPCO:
			// The element reached Unrecognized because its content did not decode.
			out.ExtendedProtocolConfigurationOptions = &ExtendedProtocolConfigurationOptions{
				Error: "failed to parse extended protocol configuration options content",
			}
		}
	}

	return out
}

func buildPDUSessionType(sessType uint8) utils.EnumField {
	return utils.NamedEnum(sessType, fgs.PDUSessionType(sessType).Name())
}

func ptr(b bool) *bool { return &b }

// pcoContainerFlags maps a UL protocol/container identifier (TS 24.008 §10.5.6.3) to
// the setter that records its presence in the rendered extended PCO.
var pcoContainerFlags = map[uint16]func(*ExtendedProtocolConfigurationOptions){
	0x0001: func(o *ExtendedProtocolConfigurationOptions) { o.PCSCFIPv6AddressRequestUL = ptr(true) },
	0x0002: func(o *ExtendedProtocolConfigurationOptions) { o.IMCNSubsystemSignalingFlagUL = ptr(true) },
	0x0003: func(o *ExtendedProtocolConfigurationOptions) { o.DNSServerIPv6AddressRequestUL = ptr(true) },
	0x0004: func(o *ExtendedProtocolConfigurationOptions) { o.NotSupportedUL = ptr(true) },
	0x0005: func(o *ExtendedProtocolConfigurationOptions) {
		o.MSSupportOfNetworkRequestedBearerControlIndicatorUL = ptr(true)
	},
	0x0007: func(o *ExtendedProtocolConfigurationOptions) { o.DSMIPv6HomeAgentAddressRequestUL = ptr(true) },
	0x0008: func(o *ExtendedProtocolConfigurationOptions) { o.DSMIPv6HomeNetworkPrefixRequestUL = ptr(true) },
	0x0009: func(o *ExtendedProtocolConfigurationOptions) { o.DSMIPv6IPv4HomeAgentAddressRequestUL = ptr(true) },
	0x000a: func(o *ExtendedProtocolConfigurationOptions) { o.IPAddressAllocationViaNASSignallingUL = ptr(true) },
	0x000b: func(o *ExtendedProtocolConfigurationOptions) { o.IPv4AddressAllocationViaDHCPv4UL = ptr(true) },
	0x000c: func(o *ExtendedProtocolConfigurationOptions) { o.PCSCFIPv4AddressRequestUL = ptr(true) },
	0x000d: func(o *ExtendedProtocolConfigurationOptions) { o.DNSServerIPv4AddressRequestUL = ptr(true) },
	0x000e: func(o *ExtendedProtocolConfigurationOptions) { o.MSISDNRequestUL = ptr(true) },
	0x000f: func(o *ExtendedProtocolConfigurationOptions) { o.IFOMSupportRequestUL = ptr(true) },
	0x0011: func(o *ExtendedProtocolConfigurationOptions) { o.MSSupportOfLocalAddressInTFTIndicatorUL = ptr(true) },
	0x0012: func(o *ExtendedProtocolConfigurationOptions) { o.PCSCFReSelectionSupportUL = ptr(true) },
	0x0013: func(o *ExtendedProtocolConfigurationOptions) { o.NBIFOMRequestIndicatorUL = ptr(true) },
	0x0014: func(o *ExtendedProtocolConfigurationOptions) { o.NBIFOMModeUL = ptr(true) },
	0x0015: func(o *ExtendedProtocolConfigurationOptions) { o.NonIPLinkMTURequestUL = ptr(true) },
	0x0016: func(o *ExtendedProtocolConfigurationOptions) { o.APNRateControlSupportIndicatorUL = ptr(true) },
	0x0017: func(o *ExtendedProtocolConfigurationOptions) { o.UEStatus3GPPPSDataOffUL = ptr(true) },
	0x0018: func(o *ExtendedProtocolConfigurationOptions) { o.ReliableDataServiceRequestIndicatorUL = ptr(true) },
	0x0019: func(o *ExtendedProtocolConfigurationOptions) {
		o.AdditionalAPNRateControlForExceptionDataSupportIndicatorUL = ptr(true)
	},
	0x001a: func(o *ExtendedProtocolConfigurationOptions) { o.PDUSessionIDUL = ptr(true) },
	0x0020: func(o *ExtendedProtocolConfigurationOptions) { o.EthernetFramePayloadMTURequestUL = ptr(true) },
	0x0021: func(o *ExtendedProtocolConfigurationOptions) { o.UnstructuredLinkMTURequestUL = ptr(true) },
	0x0022: func(o *ExtendedProtocolConfigurationOptions) { o.I5GSMCauseValueUL = ptr(true) },
	0x0023: func(o *ExtendedProtocolConfigurationOptions) {
		o.QoSRulesWithTheLengthOfTwoOctetsSupportIndicatorUL = ptr(true)
	},
	0x0024: func(o *ExtendedProtocolConfigurationOptions) {
		o.QoSFlowDescriptionsWithTheLengthOfTwoOctetsSupportIndicatorUL = ptr(true)
	},
	0x8021: func(o *ExtendedProtocolConfigurationOptions) { o.InternetProtocolControlProtocolUL = ptr(true) },
	0xc021: func(o *ExtendedProtocolConfigurationOptions) { o.LinkControlProtocolUL = ptr(true) },
	0xc023: func(o *ExtendedProtocolConfigurationOptions) { o.PushAccessControlProtocolUL = ptr(true) },
	0xc223: func(o *ExtendedProtocolConfigurationOptions) {
		o.ChallengeHandshakeAuthenticationProtocolUL = ptr(true)
	},
}

func extendedPCOFromNAS(opts nas.ProtocolConfigurationOptions) *ExtendedProtocolConfigurationOptions {
	out := &ExtendedProtocolConfigurationOptions{}

	for _, id := range opts.ContainerIDs() {
		if set, ok := pcoContainerFlags[id]; ok {
			set(out)
		} else {
			out.Error = fmt.Sprintf("unknown container ID %d", id)
		}
	}

	return out
}
