package decoder

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"github.com/omec-project/nas/security"
	"go.uber.org/zap"
)

type SecurityHeader struct {
	ProtocolDiscriminator     uint8  `json:"protocol_discriminator"`
	SecurityHeaderType        string `json:"security_header_type"`
	MessageAuthenticationCode uint32 `json:"message_authentication_code,omitempty"`
	SequenceNumber            uint8  `json:"sequence_number"`
}

type GmmHeader struct {
	MessageType string `json:"message_type"`
}

type MobileIdentity5GS struct {
	Identity string
	PLMNID   *PLMNID `json:"plmn_id,omitempty"`
	SUCI     *string `json:"suci,omitempty"`
	GUTI     *string `json:"guti,omitempty"`
	STMSI    *string `json:"s_tmsi,omitempty"`
	IMEI     *string `json:"imei,omitempty"`
	IMEISV   *string `json:"imeisv,omitempty"`
}

type IntegrityAlgorithm struct {
	NIA0 bool `json:"nia0"`
	NIA1 bool `json:"nia1"`
	NIA2 bool `json:"nia2"`
	NIA3 bool `json:"nia3"`
}

type CipheringAlgorithm struct {
	NEA0 bool `json:"nea0"`
	NEA1 bool `json:"nea1"`
	NEA2 bool `json:"nea2"`
	NEA3 bool `json:"nea3"`
}

type UESecurityCapability struct {
	IntegrityAlgorithm IntegrityAlgorithm `json:"integrity_algorithm"`
	CipheringAlgorithm CipheringAlgorithm `json:"ciphering_algorithm"`
}

type AuthenticationRequest struct {
	ExtendedProtocolDiscriminator        uint8     `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType  uint8     `json:"spare_half_octet_and_security_header_type"`
	AuthenticationRequestMessageIdentity string    `json:"authentication_request_message_identity"`
	SpareHalfOctetAndNgksi               uint8     `json:"spare_half_octet_and_ngksi"`
	ABBA                                 []uint8   `json:"abba"`
	AuthenticationParameterAUTN          [16]uint8 `json:"authentication_parameter_autn,omitempty"`
	AuthenticationParameterRAND          [16]uint8 `json:"authentication_parameter_rand,omitempty"`
	EAPMessage                           []byte    `json:"eap_message,omitempty"`
}

type RegistrationRequest struct {
	ExtendedProtocolDiscriminator       uint8                 `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                 `json:"spare_half_octet_and_security_header_type"`
	RegistrationRequestMessageIdentity  string                `json:"registration_request_message_identity"`
	NgksiAndRegistrationType5GS         uint8                 `json:"ngksi_and_registration_type_5gs"`
	MobileIdentity5GS                   MobileIdentity5GS     `json:"mobile_identity_5gs"`
	UESecurityCapability                *UESecurityCapability `json:"ue_security_capability,omitempty"`
}

type AuthenticationFailure struct {
	ExtendedProtocolDiscriminator        uint8  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType  uint8  `json:"spare_half_octet_and_security_header_type"`
	AuthenticationFailureMessageIdentity string `json:"authentication_failure_message_identity"`
	Cause5GMM                            string `json:"cause"`
}

type AuthenticationReject struct {
	ExtendedProtocolDiscriminator       uint8  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8  `json:"spare_half_octet_and_security_header_type"`
	AuthenticationRejectMessageIdentity string `json:"authentication_reject_message_identity"`
	EAPMessage                          []byte `json:"eap_message,omitempty"`
}

type AuthenticationResponseParameter struct {
	ResStar [16]uint8 `json:"res_star"`
}

type AuthenticationResponse struct {
	ExtendedProtocolDiscriminator         uint8                            `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType   uint8                            `json:"spare_half_octet_and_security_header_type"`
	AuthenticationResponseMessageIdentity string                           `json:"authentication_response_message_identity"`
	AuthenticationResponseParameter       *AuthenticationResponseParameter `json:"authentication_response_parameter,omitempty"`
	EAPMessage                            []byte                           `json:"eap_message,omitempty"`
}

type RegistrationComplete struct {
	ExtendedProtocolDiscriminator       uint8   `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8   `json:"spare_half_octet_and_security_header_type"`
	RegistrationCompleteMessageIdentity string  `json:"registration_complete_message_identity"`
	GetSORContent                       []uint8 `json:"sor_transparent_container,omitempty"`
}

type PayloadContainer struct {
	Raw        []byte      `json:"raw"`
	GsmMessage *GsmMessage `json:"gsm_message,omitempty"`
}

type ULNASTransport struct {
	ExtendedProtocolDiscriminator         uint8            `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType   uint8            `json:"spare_half_octet_and_security_header_type"`
	ULNASTRANSPORTMessageIdentity         string           `json:"ul_nas_transport_message_identity"`
	SpareHalfOctetAndPayloadContainerType uint8            `json:"spare_half_octet_and_payload_container_type"`
	PayloadContainer                      PayloadContainer `json:"payload_container"`
	PduSessionID2Value                    *uint8           `json:"pdu_session_id_2_value,omitempty"`
	OldPDUSessionID                       *uint8           `json:"old_pdu_session_id,omitempty"`
	RequestType                           *string          `json:"request_type,omitempty"`
	SNSSAI                                *SNSSAI          `json:"snssai,omitempty"`
	DNN                                   *string          `json:"dnn,omitempty"`
}

type DLNASTransport struct {
	ExtendedProtocolDiscriminator         uint8            `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType   uint8            `json:"spare_half_octet_and_security_header_type"`
	DLNASTRANSPORTMessageIdentity         string           `json:"dl_nas_transport_message_identity"`
	SpareHalfOctetAndPayloadContainerType uint8            `json:"spare_half_octet_and_payload_container_type"`
	PayloadContainer                      PayloadContainer `json:"payload_container"`
	PduSessionID2Value                    *uint8           `json:"pdu_session_id_2_value,omitempty"`
	AdditionalInformation                 *string          `json:"additional_information,omitempty"`
	Cause5GMM                             *string          `json:"cause_5gmm,omitempty"`
	BackoffTimerValue                     *uint8           `json:"backoff_timer_value,omitempty"`
	Ipaddr                                string           `json:"ip_addr,omitempty"`
}

type UplinkDataStatusPDU struct {
	PDUSessionID int  `json:"pdu_session_id"`
	Active       bool `json:"active"`
}

type PDUSessionStatusPDU struct {
	PDUSessionID int  `json:"pdu_session_id"`
	Active       bool `json:"active"`
}

type AllowedPDUSessionStatus struct {
	PDUSessionID int  `json:"pdu_session_id"`
	Active       bool `json:"active"`
}

type ServiceRequest struct {
	ExtendedProtocolDiscriminator       uint8                     `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                     `json:"spare_half_octet_and_security_header_type"`
	ServiceRequestMessageIdentity       string                    `json:"service_request_message_identity"`
	ServiceTypeAndNgksi                 string                    `json:"service_type_and_ngksi"`
	TMSI5GS                             TMSI5GS                   `json:"tmsi_5gs,omitempty"`
	UplinkDataStatus                    []UplinkDataStatusPDU     `json:"uplink_data_status,omitempty"`
	PDUSessionStatus                    []PDUSessionStatusPDU     `json:"pdu_session_status,omitempty"`
	AllowedPDUSessionStatus             []AllowedPDUSessionStatus `json:"allowed_pdu_session_status,omitempty"`
	NASMessageContainer                 []byte                    `json:"nas_message_container,omitempty"`
}

type PDUSessionReactivateResultPDU struct {
	PDUSessionID int  `json:"pdu_session_id"`
	Active       bool `json:"active"`
}

type PDUSessionCause struct {
	PDUSessionID uint8  `json:"pdu_session_id"`
	Cause        string `json:"cause"`
}

type ServiceAccept struct {
	ExtendedProtocolDiscriminator          uint8                           `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType    uint8                           `json:"spare_half_octet_and_security_header_type"`
	ServiceAcceptMessageIdentity           string                          `json:"service_accept_message_identity"`
	PDUSessionStatus                       []PDUSessionStatusPDU           `json:"pdu_session_status,omitempty"`
	PDUSessionReactivationResult           []PDUSessionReactivateResultPDU `json:"pdu_session_reactivation_result,omitempty"`
	PDUSessionReactivationResultErrorCause []PDUSessionCause               `json:"pdu_session_reactivation_result_error_cause,omitempty"`
	EAPMessage                             []byte                          `json:"eap_message,omitempty"`
}

type ServiceReject struct {
	ExtendedProtocolDiscriminator       uint8                 `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                 `json:"spare_half_octet_and_security_header_type"`
	ServiceRejectMessageIdentity        string                `json:"service_reject_message_identity"`
	Cause5GMM                           string                `json:"cause"`
	PDUSessionStatus                    []PDUSessionStatusPDU `json:"pdu_session_status,omitempty"`
	T3346Value                          *uint8                `json:"t3346_value,omitempty"`
	EAPMessage                          []byte                `json:"eap_message,omitempty"`
}

type Additional5GSecurityInformation struct {
	RINMR uint8 `json:"rinmr"`
	HDP   uint8 `json:"hdp"`
}

type SelectedNASSecurityAlgorithms struct {
	Integrity string `json:"integrity"`
	Ciphering string `json:"ciphering"`
}

type SecurityModeCommand struct {
	ExtendedProtocolDiscriminator       uint8                            `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                            `json:"spare_half_octet_and_security_header_type"`
	SecurityModeCommandMessageIdentity  string                           `json:"security_mode_command_message_identity"`
	SelectedNASSecurityAlgorithms       SelectedNASSecurityAlgorithms    `json:"selected_nas_security_algorithms"`
	SpareHalfOctetAndNgksi              uint8                            `json:"spare_half_octet_and_ngksi"`
	ReplayedUESecurityCapabilities      UESecurityCapability             `json:"replayed_ue_security_capabilities"`
	IMEISVRequest                       *string                          `json:"imeisv_request,omitempty"`
	SelectedEPSNASSecurityAlgorithms    *string                          `json:"selected_eps_nas_security_algorithms,omitempty"`
	Additional5GSecurityInformation     *Additional5GSecurityInformation `json:"additional_5g_security_information,omitempty"`
	EAPMessage                          []byte                           `json:"eap_message,omitempty"`
	ABBA                                []uint8                          `json:"abba,omitempty"`
	ReplayedS1UESecurityCapabilities    *UESecurityCapability            `json:"replayed_s1_ue_security_capabilities,omitempty"`
}

type SecurityModeComplete struct {
	ExtendedProtocolDiscriminator       uint8   `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8   `json:"spare_half_octet_and_security_header_type"`
	SecurityModeCompleteMessageIdentity string  `json:"security_mode_complete_message_identity"`
	IMEISV                              *string `json:"imeisv,omitempty"`
	NASMessageContainer                 []byte  `json:"nas_message_container,omitempty"`
}

// nasType.ExtendedProtocolDiscriminator
// 	nasType.SpareHalfOctetAndSecurityHeaderType
// 	nasType.RegistrationAcceptMessageIdentity
// 	nasType.RegistrationResult5GS
// 	*nasType.GUTI5G
// 	*nasType.EquivalentPlmns
// 	*nasType.TAIList
// 	*nasType.AllowedNSSAI
// 	*nasType.RejectedNSSAI
// 	*nasType.ConfiguredNSSAI
// 	*nasType.NetworkFeatureSupport5GS
// 	*nasType.PDUSessionStatus
// 	*nasType.PDUSessionReactivationResult
// 	*nasType.PDUSessionReactivationResultErrorCause
// 	*nasType.LADNInformation
// 	*nasType.MICOIndication
// 	*nasType.NetworkSlicingIndication
// 	*nasType.ServiceAreaList
// 	*nasType.T3512Value
// 	*nasType.Non3GppDeregistrationTimerValue
// 	*nasType.T3502Value
// 	*nasType.EmergencyNumberList
// 	*nasType.ExtendedEmergencyNumberList
// 	*nasType.SORTransparentContainer
// 	*nasType.EAPMessage
// 	*nasType.NSSAIInclusionMode
// 	*nasType.OperatordefinedAccessCategoryDefinitions
// 	*nasType.NegotiatedDRXParameters

type NetworkFeatureSupport5GS struct {
	Emc          uint8 `json:"emc"`
	EmcN3        uint8 `json:"emc_n3"`
	Emf          uint8 `json:"emf"`
	ImsVoPS      uint8 `json:"ims_vops"`
	IwkN26       uint8 `json:"iwk_n26"`
	Mcsi         uint8 `json:"mcsi"`
	Mpsi         uint8 `json:"mpsi"`
	IMSVoPS3GPP  uint8 `json:"ims_vops_3gpp"`
	IMSVoPSN3GPP uint8 `json:"ims_vops_n3gpp"`
}

type RegistrationAccept struct {
	ExtendedProtocolDiscriminator       uint8                     `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                     `json:"spare_half_octet_and_security_header_type"`
	RegistrationAcceptMessageIdentity   string                    `json:"registration_accept_message_identity"`
	RegistrationResult5GS               string                    `json:"registration_result_5gs"`
	GUTI5G                              *string                   `json:"guti_5g,omitempty"`
	EquivalentPLMNs                     []PLMNID                  `json:"equivalent_plmns,omitempty"`
	TAIList                             []TAI                     `json:"tai_list,omitempty"`
	AllowedNSSAI                        []SNSSAI                  `json:"allowed_nssai,omitempty"`
	NetworkFeatureSupport5GS            *NetworkFeatureSupport5GS `json:"network_feature_support_5gs,omitempty"`
}

type RegistrationReject struct {
	ExtendedProtocolDiscriminator       uint8  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8  `json:"spare_half_octet_and_security_header_type"`
	RegistrationRejectMessageIdentity   string `json:"registration_reject_message_identity"`
	Cause5GMM                           string `json:"cause_5gmm"`
}

type GmmMessage struct {
	GmmHeader              GmmHeader               `json:"gmm_header"`
	RegistrationRequest    *RegistrationRequest    `json:"registration_request,omitempty"`
	RegistrationAccept     *RegistrationAccept     `json:"registration_accept,omitempty"`
	RegistrationReject     *RegistrationReject     `json:"registration_reject,omitempty"`
	RegistrationComplete   *RegistrationComplete   `json:"registration_complete,omitempty"`
	AuthenticationRequest  *AuthenticationRequest  `json:"authentication_request,omitempty"`
	AuthenticationFailure  *AuthenticationFailure  `json:"authentication_failure,omitempty"`
	AuthenticationReject   *AuthenticationReject   `json:"authentication_reject,omitempty"`
	AuthenticationResponse *AuthenticationResponse `json:"authentication_response,omitempty"`
	ULNASTransport         *ULNASTransport         `json:"ul_nas_transport,omitempty"`
	DLNASTransport         *DLNASTransport         `json:"dl_nas_transport,omitempty"`
	SecurityModeCommand    *SecurityModeCommand    `json:"security_mode_command,omitempty"`
	SecurityModeComplete   *SecurityModeComplete   `json:"security_mode_complete,omitempty"`
	ServiceRequest         *ServiceRequest         `json:"service_request,omitempty"`
	ServiceAccept          *ServiceAccept          `json:"service_accept,omitempty"`
	ServiceReject          *ServiceReject          `json:"service_reject,omitempty"`
}

type GsmHeader struct {
	MessageType string `json:"message_type"`
}

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

// nasType.ExtendedProtocolDiscriminator
// 	nasType.PDUSessionID
// 	nasType.PTI
// 	nasType.PDUSESSIONESTABLISHMENTACCEPTMessageIdentity
// 	nasType.SelectedSSCModeAndSelectedPDUSessionType
// 	nasType.AuthorizedQosRules
// 	nasType.SessionAMBR
// 	*nasType.Cause5GSM
// 	*nasType.PDUAddress
// 	*nasType.RQTimerValue
// 	*nasType.SNSSAI
// 	*nasType.AlwaysonPDUSessionIndication
// 	*nasType.MappedEPSBearerContexts
// 	*nasType.EAPMessage
// 	*nasType.AuthorizedQosFlowDescriptions
// 	*nasType.ExtendedProtocolConfigurationOptions
// 	*nasType.DNN

type PDUSessionEstablishmentAccept struct {
	ExtendedProtocolDiscriminator                uint8     `json:"extended_protocol_discriminator"`
	PDUSessionID                                 uint8     `json:"pdu_session_id"`
	PTI                                          uint8     `json:"pti"`
	PDUSESSIONESTABLISHMENTACCEPTMessageIdentity uint8     `json:"pdu_session_establishment_accept_message_identity"`
	SelectedSSCMode                              uint8     `json:"selected_ssc_mode"`
	SelectedPDUSessionType                       string    `json:"selected_pdu_session_type"`
	AuthorizedQosRules                           []QosRule `json:"authorized_qos_rules"`
	SessionAMBR                                  string    `json:"session_ambr"`
}

type GsmMessage struct {
	GsmHeader                      GsmHeader                       `json:"gsm_header"`
	PDUSessionEstablishmentRequest *PDUSessionEstablishmentRequest `json:"pdu_session_establishment_request,omitempty"`
	PDUSessionEstablishmentAccept  *PDUSessionEstablishmentAccept  `json:"pdu_session_establishment_accept,omitempty"`
}

type NASMessage struct {
	SecurityHeader SecurityHeader `json:"security_header"`
	GmmMessage     *GmmMessage    `json:"gmm_message,omitempty"`
	GsmMessage     *GsmMessage    `json:"gsm_message,omitempty"`
}

type NasContextInfo struct {
	Direction   Direction
	AMFUENGAPID int64
}

func DecodeNASMessage(raw []byte, nasContextInfo *NasContextInfo) (*NASMessage, error) {
	nasMsg := new(NASMessage)

	msg, err := decodeNAS(raw, nasContextInfo)
	if err != nil {
		return nil, err
	}

	nasMsg.SecurityHeader = buildSecurityHeader(msg)

	epd := nas.GetEPD(raw)
	switch epd {
	case nasMessage.Epd5GSMobilityManagementMessage:
		nasMsg.GmmMessage = buildGmmMessage(msg.GmmMessage)
	case nasMessage.Epd5GSSessionManagementMessage:
		nasMsg.GsmMessage = buildGsmMessage(msg.GsmMessage)
	default:
		return nil, fmt.Errorf("unsupported EPD: %d", epd)
	}

	return nasMsg, nil
}

func buildGmmMessage(msg *nas.GmmMessage) *GmmMessage {
	if msg == nil {
		return nil
	}
	gmmMessage := &GmmMessage{
		GmmHeader: GmmHeader{
			MessageType: getGmmMessageType(msg),
		},
	}

	switch msg.GetMessageType() {
	case nas.MsgTypeRegistrationRequest:
		gmmMessage.RegistrationRequest = buildRegistrationRequest(msg.RegistrationRequest)
		return gmmMessage
	case nas.MsgTypeRegistrationAccept:
		gmmMessage.RegistrationAccept = buildRegistrationAccept(msg.RegistrationAccept)
		return gmmMessage
	case nas.MsgTypeRegistrationReject:
		gmmMessage.RegistrationReject = buildRegistrationReject(msg.RegistrationReject)
		return gmmMessage
	case nas.MsgTypeRegistrationComplete:
		gmmMessage.RegistrationComplete = buildRegistrationComplete(msg.RegistrationComplete)
		return gmmMessage
	case nas.MsgTypeAuthenticationRequest:
		gmmMessage.AuthenticationRequest = buildAuthenticationRequest(msg.AuthenticationRequest)
		return gmmMessage
	case nas.MsgTypeAuthenticationFailure:
		gmmMessage.AuthenticationFailure = buildAuthenticationFailure(msg.AuthenticationFailure)
		return gmmMessage
	case nas.MsgTypeAuthenticationReject:
		gmmMessage.AuthenticationReject = buildAuthenticationReject(msg.AuthenticationReject)
		return gmmMessage
	case nas.MsgTypeAuthenticationResponse:
		gmmMessage.AuthenticationResponse = buildAuthenticationResponse(msg.AuthenticationResponse)
		return gmmMessage
	case nas.MsgTypeULNASTransport:
		gmmMessage.ULNASTransport = buildULNASTransport(msg.ULNASTransport)
		return gmmMessage
	case nas.MsgTypeDLNASTransport:
		gmmMessage.DLNASTransport = buildDLNASTransport(msg.DLNASTransport)
		return gmmMessage
	case nas.MsgTypeSecurityModeCommand:
		gmmMessage.SecurityModeCommand = buildSecurityModeCommand(msg.SecurityModeCommand)
		return gmmMessage
	case nas.MsgTypeSecurityModeComplete:
		gmmMessage.SecurityModeComplete = buildSecurityModeComplete(msg.SecurityModeComplete)
		return gmmMessage
	case nas.MsgTypeServiceRequest:
		gmmMessage.ServiceRequest = buildServiceRequest(msg.ServiceRequest)
		return gmmMessage
	case nas.MsgTypeServiceAccept:
		gmmMessage.ServiceAccept = buildServiceAccept(msg.ServiceAccept)
		return gmmMessage
	case nas.MsgTypeServiceReject:
		gmmMessage.ServiceReject = buildServiceReject(msg.ServiceReject)
		return gmmMessage
	default:
		logger.EllaLog.Warn("GMM message type not fully implemented", zap.String("message_type", gmmMessage.GmmHeader.MessageType))
		return gmmMessage
	}
}

func buildRegistrationReject(msg *nasMessage.RegistrationReject) *RegistrationReject {
	if msg == nil {
		return nil
	}
	regRej := &RegistrationReject{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		RegistrationRejectMessageIdentity:   nas.MessageName(msg.RegistrationRejectMessageIdentity.Octet),
		Cause5GMM:                           nasMessage.Cause5GMMToString(msg.Cause5GMM.Octet),
	}

	if msg.T3346Value != nil {
		logger.EllaLog.Warn("T3346Value in RegistrationReject is not implemented")
	}

	if msg.T3502Value != nil {
		logger.EllaLog.Warn("T3502Value in RegistrationReject is not implemented")
	}

	if msg.EAPMessage != nil {
		logger.EllaLog.Warn("EAPMessage in RegistrationReject is not implemented")
	}

	return regRej
}

func buildAuthenticationResponse(msg *nasMessage.AuthenticationResponse) *AuthenticationResponse {
	if msg == nil {
		return nil
	}

	authResp := &AuthenticationResponse{
		ExtendedProtocolDiscriminator:         msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:   msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		AuthenticationResponseMessageIdentity: nas.MessageName(msg.AuthenticationResponseMessageIdentity.Octet),
	}

	if msg.AuthenticationResponseParameter != nil {
		authResp.AuthenticationResponseParameter = &AuthenticationResponseParameter{
			ResStar: msg.AuthenticationResponseParameter.GetRES(),
		}
	}

	if msg.EAPMessage != nil {
		authResp.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return authResp
}

type TMSI5GS struct {
	TypeOfIdentity string   `json:"type_of_identity"`
	AMFSetID       uint16   `json:"amf_set_id"`
	AMFPointer     uint8    `json:"amf_pointer"`
	TMSI5G         [4]uint8 `json:"tmsi_5g"`
}

func buildTMSI5GS(tmsi5gs nasType.TMSI5GS) TMSI5GS {
	var typeOfIdentity string
	switch tmsi5gs.GetTypeOfIdentity() {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		typeOfIdentity = "NoIdentity"
	case nasMessage.MobileIdentity5GSTypeSuci:
		typeOfIdentity = "Suci"
	case nasMessage.MobileIdentity5GSType5gGuti:
		typeOfIdentity = "5gGuti"
	case nasMessage.MobileIdentity5GSTypeImei:
		typeOfIdentity = "Imei"
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		typeOfIdentity = "5gSTmsi"
	case nasMessage.MobileIdentity5GSTypeImeisv:
		typeOfIdentity = "Imeisv"
	default:
		typeOfIdentity = fmt.Sprintf("Unknown(%d)", tmsi5gs.GetTypeOfIdentity())
	}

	return TMSI5GS{
		TypeOfIdentity: typeOfIdentity,
		AMFSetID:       tmsi5gs.GetAMFSetID(),
		AMFPointer:     tmsi5gs.GetAMFPointer(),
		TMSI5G:         tmsi5gs.GetTMSI5G(),
	}
}

func buildServiceReject(msg *nasMessage.ServiceReject) *ServiceReject {
	if msg == nil {
		return nil
	}

	serviceReject := &ServiceReject{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		ServiceRejectMessageIdentity:        nas.MessageName(msg.ServiceRejectMessageIdentity.Octet),
		Cause5GMM:                           nasMessage.Cause5GMMToString(msg.Cause5GMM.Octet),
	}

	if msg.PDUSessionStatus != nil {
		pduSessionStatus := []PDUSessionStatusPDU{}
		psiArray := nasConvert.PSIToBooleanArray(msg.PDUSessionStatus.Buffer)
		for pduSessionID, isActive := range psiArray {
			pduSessionStatus = append(pduSessionStatus, PDUSessionStatusPDU{
				PDUSessionID: pduSessionID,
				Active:       isActive,
			})
		}
		serviceReject.PDUSessionStatus = pduSessionStatus
	}

	if msg.T3346Value != nil {
		t3346Value := msg.T3346Value.GetGPRSTimer2Value()
		serviceReject.T3346Value = &t3346Value
	}

	if msg.EAPMessage != nil {
		serviceReject.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return serviceReject
}

func buildServiceAccept(msg *nasMessage.ServiceAccept) *ServiceAccept {
	if msg == nil {
		return nil
	}

	serviceAccept := &ServiceAccept{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		ServiceAcceptMessageIdentity:        nas.MessageName(msg.ServiceAcceptMessageIdentity.Octet),
	}

	if msg.PDUSessionStatus != nil {
		pduSessionStatus := []PDUSessionStatusPDU{}
		psiArray := nasConvert.PSIToBooleanArray(msg.PDUSessionStatus.Buffer)
		for pduSessionID, isActive := range psiArray {
			pduSessionStatus = append(pduSessionStatus, PDUSessionStatusPDU{
				PDUSessionID: pduSessionID,
				Active:       isActive,
			})
		}
		serviceAccept.PDUSessionStatus = pduSessionStatus
	}

	if msg.PDUSessionReactivationResult != nil {
		pduSessionReactivationResult := []PDUSessionReactivateResultPDU{}
		psiArray := nasConvert.PSIToBooleanArray(msg.PDUSessionReactivationResult.Buffer)
		for pduSessionID, isActive := range psiArray {
			pduSessionReactivationResult = append(pduSessionReactivationResult, PDUSessionReactivateResultPDU{
				PDUSessionID: pduSessionID,
				Active:       isActive,
			})
		}
		serviceAccept.PDUSessionReactivationResult = pduSessionReactivationResult
	}

	if msg.PDUSessionReactivationResultErrorCause != nil {
		logger.EllaLog.Warn("PDUSessionReactivationResultErrorCause not yet implemented")
		// Cause5GMMToString
		pduSessionIDAndCause := msg.PDUSessionReactivationResultErrorCause.GetPDUSessionIDAndCauseValue()
		pduSessionIDs, causes := bufToPDUSessionReactivationResultErrorCause(pduSessionIDAndCause)
		if len(pduSessionIDs) != len(causes) {
			logger.EllaLog.Warn("PDUSessionReactivationResultErrorCause: invalid length")
		} else {
			var pduSessionCauses []PDUSessionCause
			for i := range pduSessionIDs {
				pduSessionCauses = append(pduSessionCauses, PDUSessionCause{
					PDUSessionID: pduSessionIDs[i],
					Cause:        nasMessage.Cause5GMMToString(causes[i]),
				})
			}
			serviceAccept.PDUSessionReactivationResultErrorCause = pduSessionCauses
		}
	}

	if msg.EAPMessage != nil {
		serviceAccept.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return serviceAccept
}

func bufToPDUSessionReactivationResultErrorCause(buf []uint8) (errPduSessionId, errCause []uint8) {
	if len(buf)%2 != 0 {
		return nil, nil
	}

	n := len(buf) / 2
	errPduSessionId = make([]uint8, 0, n)
	errCause = make([]uint8, 0, n)

	for i := 0; i < len(buf); i += 2 {
		errPduSessionId = append(errPduSessionId, buf[i])
		errCause = append(errCause, buf[i+1])
	}
	return
}

func buildServiceRequest(msg *nasMessage.ServiceRequest) *ServiceRequest {
	if msg == nil {
		return nil
	}

	serviceRequest := &ServiceRequest{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		ServiceRequestMessageIdentity:       nas.MessageName(msg.ServiceRequestMessageIdentity.Octet),
		ServiceTypeAndNgksi:                 nas.MessageName(msg.ServiceTypeAndNgksi.Octet),
		TMSI5GS:                             buildTMSI5GS(msg.TMSI5GS),
	}

	if msg.UplinkDataStatus != nil {
		uplinkDataStatus := []UplinkDataStatusPDU{}
		uplinkDataPsi := nasConvert.PSIToBooleanArray(msg.UplinkDataStatus.Buffer)
		for pduSessionID, hasUplinkData := range uplinkDataPsi {
			uplinkDataStatus = append(uplinkDataStatus, UplinkDataStatusPDU{
				PDUSessionID: pduSessionID,
				Active:       hasUplinkData,
			})
		}
		serviceRequest.UplinkDataStatus = uplinkDataStatus
	}

	if msg.PDUSessionStatus != nil {
		pduSessionStatus := []PDUSessionStatusPDU{}
		psiArray := nasConvert.PSIToBooleanArray(msg.PDUSessionStatus.Buffer)
		for pduSessionID, isActive := range psiArray {
			pduSessionStatus = append(pduSessionStatus, PDUSessionStatusPDU{
				PDUSessionID: pduSessionID,
				Active:       isActive,
			})
		}
		serviceRequest.PDUSessionStatus = pduSessionStatus
	}

	if msg.AllowedPDUSessionStatus != nil {
		allowedPduSessionStatus := []AllowedPDUSessionStatus{}
		allowedPsis := nasConvert.PSIToBooleanArray(msg.AllowedPDUSessionStatus.Buffer)
		for pduSessionID, isAllowed := range allowedPsis {
			allowedPduSessionStatus = append(allowedPduSessionStatus, AllowedPDUSessionStatus{
				PDUSessionID: pduSessionID,
				Active:       isAllowed,
			})
		}
		serviceRequest.AllowedPDUSessionStatus = allowedPduSessionStatus
	}

	if msg.NASMessageContainer != nil {
		serviceRequest.NASMessageContainer = msg.NASMessageContainer.GetNASMessageContainerContents()
	}

	return serviceRequest
}

func buildSecurityModeComplete(msg *nasMessage.SecurityModeComplete) *SecurityModeComplete {
	if msg == nil {
		return nil
	}

	securityModeComplete := &SecurityModeComplete{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		SecurityModeCompleteMessageIdentity: nas.MessageName(msg.SecurityModeCompleteMessageIdentity.Octet),
	}

	if msg.IMEISV != nil {
		pei := nasConvert.PeiToString(msg.IMEISV.Octet[:])
		securityModeComplete.IMEISV = &pei
	}

	if msg.NASMessageContainer != nil {
		securityModeComplete.NASMessageContainer = msg.NASMessageContainer.GetNASMessageContainerContents()
	}

	return securityModeComplete
}

func buildSecurityModeCommand(msg *nasMessage.SecurityModeCommand) *SecurityModeCommand {
	if msg == nil {
		return nil
	}

	securityModeCommand := &SecurityModeCommand{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		SecurityModeCommandMessageIdentity:  nas.MessageName(msg.SecurityModeCommandMessageIdentity.Octet),
		SelectedNASSecurityAlgorithms:       buildSelectedNASSecurityAlgorithms(msg.SelectedNASSecurityAlgorithms),
		SpareHalfOctetAndNgksi:              msg.SpareHalfOctetAndNgksi.Octet,
		ReplayedUESecurityCapabilities:      *buildReplayedUESecurityCapability(msg.ReplayedUESecurityCapabilities),
	}

	if msg.IMEISVRequest != nil {
		value := buildIMEISVRequest(*msg.IMEISVRequest)
		securityModeCommand.IMEISVRequest = &value
	}

	if msg.SelectedEPSNASSecurityAlgorithms != nil {
		algo := getIntegrity(msg.SelectedEPSNASSecurityAlgorithms.GetTypeOfIntegrityProtectionAlgorithm())
		securityModeCommand.SelectedEPSNASSecurityAlgorithms = &algo
	}

	if msg.Additional5GSecurityInformation != nil {
		securityModeCommand.Additional5GSecurityInformation = &Additional5GSecurityInformation{
			RINMR: msg.Additional5GSecurityInformation.GetRINMR(),
			HDP:   msg.Additional5GSecurityInformation.GetHDP(),
		}
	}

	if msg.EAPMessage != nil {
		securityModeCommand.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	if msg.ABBA != nil {
		securityModeCommand.ABBA = msg.ABBA.GetABBAContents()
	}

	if msg.ReplayedS1UESecurityCapabilities != nil {
		logger.EllaLog.Warn("ReplayedS1UESecurityCapabilities not yet implemented")
	}

	return securityModeCommand
}

func buildSelectedNASSecurityAlgorithms(msg nasType.SelectedNASSecurityAlgorithms) SelectedNASSecurityAlgorithms {
	return SelectedNASSecurityAlgorithms{
		Integrity: getIntegrity(msg.GetTypeOfIntegrityProtectionAlgorithm()),
		Ciphering: getCiphering(msg.GetTypeOfCipheringAlgorithm()),
	}
}

func getIntegrity(value uint8) string {
	switch value {
	case security.AlgIntegrity128NIA0:
		return "NIA0"
	case security.AlgIntegrity128NIA1:
		return "NIA1"
	case security.AlgIntegrity128NIA2:
		return "NIA2"
	case security.AlgIntegrity128NIA3:
		return "NIA3"
	default:
		return fmt.Sprintf("Unknown(%d)", value)
	}
}

func getCiphering(value uint8) string {
	switch value {
	case security.AlgCiphering128NEA0:
		return "NEA0"
	case security.AlgCiphering128NEA1:
		return "NEA1"
	case security.AlgCiphering128NEA2:
		return "NEA2"
	case security.AlgCiphering128NEA3:
		return "NEA3"
	default:
		return fmt.Sprintf("Unknown(%d)", value)
	}
}

func buildIMEISVRequest(msg nasType.IMEISVRequest) string {
	switch msg.GetIMEISVRequestValue() {
	case nasMessage.IMEISVNotRequested:
		return "NotRequested"
	case nasMessage.IMEISVRequested:
		return "Requested"
	default:
		return fmt.Sprintf("Unknown(%d)", msg.GetIMEISVRequestValue())
	}
}

func buildDLNASTransport(msg *nasMessage.DLNASTransport) *DLNASTransport {
	if msg == nil {
		return nil
	}

	dlNasTransport := &DLNASTransport{
		ExtendedProtocolDiscriminator:         msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:   msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		DLNASTRANSPORTMessageIdentity:         nas.MessageName(msg.DLNASTRANSPORTMessageIdentity.Octet),
		SpareHalfOctetAndPayloadContainerType: msg.SpareHalfOctetAndPayloadContainerType.Octet,
		Ipaddr:                                msg.Ipaddr,
	}

	dlNasTransport.PayloadContainer = buildDLNASPayloadContainer(msg)

	if msg.PduSessionID2Value != nil {
		value := msg.PduSessionID2Value.GetPduSessionID2Value()
		dlNasTransport.PduSessionID2Value = &value
	}

	if msg.AdditionalInformation != nil {
		logger.EllaLog.Warn("AdditionalInformation not yet implemented")
	}

	if msg.BackoffTimerValue != nil {
		backoffTimerValue := msg.BackoffTimerValue.GetUnitTimerValue()
		dlNasTransport.BackoffTimerValue = &backoffTimerValue
	}

	if msg.Cause5GMM != nil {
		cause := nasMessage.Cause5GMMToString(msg.Cause5GMM.GetCauseValue())
		dlNasTransport.Cause5GMM = &cause
	}

	return dlNasTransport
}

func decodeGSMMessage(raw []byte) (*GsmMessage, error) {
	m := nas.NewMessage()
	err := m.GsmMessageDecode(&raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode N1 SM message in UL NAS Transport Payload Container: %w", err)
	}

	gsmMessage := &GsmMessage{
		GsmHeader: GsmHeader{
			MessageType: getGsmMessageType(m.GsmMessage),
		},
	}

	switch m.GsmMessage.GetMessageType() {
	case nas.MsgTypePDUSessionEstablishmentRequest:
		gsmMessage.PDUSessionEstablishmentRequest = buildPDUSessionEstablishmentRequest(m.GsmMessage.PDUSessionEstablishmentRequest)
	case nas.MsgTypePDUSessionEstablishmentAccept:
		gsmMessage.PDUSessionEstablishmentAccept = buildPDUSessionEstablishmentAccept(m.GsmMessage.PDUSessionEstablishmentAccept)
	default:
		logger.EllaLog.Warn("GSM message type not yet implemented", zap.String("message_type", gsmMessage.GsmHeader.MessageType))
	}

	return gsmMessage, nil
}

func buildPDUSessionType(sessType uint8) string {
	switch sessType {
	case nasMessage.PDUSessionTypeIPv4:
		return "IPv4"
	case nasMessage.PDUSessionTypeIPv6:
		return "IPv6"
	case nasMessage.PDUSessionTypeIPv4IPv6:
		return "IPv4v6"
	case nasMessage.PDUSessionTypeUnstructured:
		return "Unstructured"
	case nasMessage.PDUSessionTypeEthernet:
		return "Ethernet"
	default:
		return fmt.Sprintf("Unknown(%d)", sessType)
	}
}

// func buildAuthorizedQosRules(rules nasType.AuthorizedQosRules) []QosRule {
// 	qosRulesBytes := rules.GetQosRule()

// 	qosRules, err := unmarshalQoSRules(qosRulesBytes)
// 	if err != nil {
// 		logger.EllaLog.Warn("failed to unmarshal authorized QoS rules", zap.Error(err))
// 		return nil
// 	}

// 	return qosRules
// }

func buildPDUSessionEstablishmentAccept(msg *nasMessage.PDUSessionEstablishmentAccept) *PDUSessionEstablishmentAccept {
	if msg == nil {
		return nil
	}

	estAcc := &PDUSessionEstablishmentAccept{
		ExtendedProtocolDiscriminator: msg.ExtendedProtocolDiscriminator.Octet,
		PDUSessionID:                  msg.PDUSessionID.GetPDUSessionID(),
		PTI:                           msg.PTI.GetPTI(),
		PDUSESSIONESTABLISHMENTACCEPTMessageIdentity: msg.PDUSESSIONESTABLISHMENTACCEPTMessageIdentity.GetMessageType(),
		SelectedSSCMode:        msg.SelectedSSCModeAndSelectedPDUSessionType.GetSSCMode(),
		SelectedPDUSessionType: buildPDUSessionType(msg.SelectedSSCModeAndSelectedPDUSessionType.GetPDUSessionType()),
		// AuthorizedQosRules:     buildAuthorizedQosRules(msg.AuthorizedQosRules),
	}

	// 	nasType.SessionAMBR
	// 	*nasType.Cause5GSM
	// 	*nasType.PDUAddress
	// 	*nasType.RQTimerValue
	// 	*nasType.SNSSAI
	// 	*nasType.AlwaysonPDUSessionIndication
	// 	*nasType.MappedEPSBearerContexts
	// 	*nasType.EAPMessage
	// 	*nasType.AuthorizedQosFlowDescriptions
	// 	*nasType.ExtendedProtocolConfigurationOptions
	// 	*nasType.DNN

	return estAcc
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

func buildCapability5GSM(msg nasType.Capability5GSM) *Capability5GSM {
	return &Capability5GSM{
		RqoS:   msg.GetRqoS(),
		MH6PDU: msg.GetMH6PDU(),
	}
}

func buildDLNASPayloadContainer(msg *nasMessage.DLNASTransport) PayloadContainer {
	containerType := msg.GetPayloadContainerType()

	payloadContainer := PayloadContainer{
		Raw: msg.GetPayloadContainerContents(),
	}

	if containerType != nasMessage.PayloadContainerTypeN1SMInfo {
		logger.EllaLog.Warn("Payload container type not yet implemented", zap.Uint8("type", containerType))
		return payloadContainer
	}

	rawBytes := msg.GetPayloadContainerContents()

	gsmMessage, err := decodeGSMMessage(rawBytes)
	if err != nil {
		logger.EllaLog.Warn("Failed to decode N1 SM message in DL NAS Transport Payload Container", zap.Error(err))
		return payloadContainer
	}

	payloadContainer.GsmMessage = gsmMessage

	return payloadContainer
}

func buildULNASPayloadContainer(msg *nasMessage.ULNASTransport) PayloadContainer {
	containerType := msg.GetPayloadContainerType()

	payloadContainer := PayloadContainer{
		Raw: msg.GetPayloadContainerContents(),
	}

	if containerType != nasMessage.PayloadContainerTypeN1SMInfo {
		logger.EllaLog.Warn("Payload container type not yet implemented", zap.Uint8("type", containerType))
		return payloadContainer
	}

	rawBytes := msg.GetPayloadContainerContents()

	gsmMessage, err := decodeGSMMessage(rawBytes)
	if err != nil {
		logger.EllaLog.Warn("Failed to decode N1 SM message in UL NAS Transport Payload Container", zap.Error(err))
		return payloadContainer
	}

	payloadContainer.GsmMessage = gsmMessage

	return payloadContainer
}

func buildULNASTransport(msg *nasMessage.ULNASTransport) *ULNASTransport {
	if msg == nil {
		return nil
	}

	ulNasTransport := &ULNASTransport{
		ExtendedProtocolDiscriminator:         msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:   msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		ULNASTRANSPORTMessageIdentity:         nas.MessageName(msg.ULNASTRANSPORTMessageIdentity.Octet),
		SpareHalfOctetAndPayloadContainerType: msg.SpareHalfOctetAndPayloadContainerType.Octet,
	}

	ulNasTransport.PayloadContainer = buildULNASPayloadContainer(msg)

	if msg.PduSessionID2Value != nil {
		value := msg.PduSessionID2Value.GetPduSessionID2Value()
		ulNasTransport.PduSessionID2Value = &value
	}

	if msg.OldPDUSessionID != nil {
		value := msg.OldPDUSessionID.GetOldPDUSessionID()
		ulNasTransport.OldPDUSessionID = &value
	}

	if msg.RequestType != nil {
		value := ""
		switch msg.RequestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialRequest:
			value = "InitialRequest"
		case nasMessage.ULNASTransportRequestTypeExistingPduSession:
			value = "ExistingPduSession"
		case nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest:
			value = "InitialEmergencyRequest"
		case nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession:
			value = "ExistingEmergencyPduSession"
		case nasMessage.ULNASTransportRequestTypeModificationRequest:
			value = "ModificationRequest"
		case nasMessage.ULNASTransportRequestTypeReserved:
			value = "Reserved"
		}
		ulNasTransport.RequestType = &value
	}

	if msg.SNSSAI != nil {
		snssai := snssaiToModels(msg.SNSSAI)
		ulNasTransport.SNSSAI = &snssai
	}

	if msg.DNN != nil {
		dnn := string(msg.DNN.GetDNN())
		ulNasTransport.DNN = &dnn
	}

	if msg.AdditionalInformation != nil {
		logger.EllaLog.Warn("AdditionalInformation not yet implemented")
	}

	return ulNasTransport
}

func snssaiToModels(n *nasType.SNSSAI) SNSSAI {
	var out SNSSAI
	out.SST = int32(n.GetSST())

	if n.Len >= 4 {
		sd := n.Octet[1:4] // 3 bytes following SST
		sdStr := strings.ToUpper(hex.EncodeToString(sd))
		out.SD = &sdStr
	} else {
		out.SD = nil
	}

	return out
}

func buildAuthenticationReject(msg *nasMessage.AuthenticationReject) *AuthenticationReject {
	if msg == nil {
		return nil
	}

	authReject := &AuthenticationReject{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		AuthenticationRejectMessageIdentity: nas.MessageName(msg.AuthenticationRejectMessageIdentity.Octet),
	}

	if msg.EAPMessage != nil {
		authReject.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return authReject
}

func buildAuthenticationFailure(msg *nasMessage.AuthenticationFailure) *AuthenticationFailure {
	if msg == nil {
		return nil
	}

	authFailure := &AuthenticationFailure{
		ExtendedProtocolDiscriminator:        msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:  msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		AuthenticationFailureMessageIdentity: nas.MessageName(msg.AuthenticationFailureMessageIdentity.Octet),
		Cause5GMM:                            nasMessage.Cause5GMMToString(msg.Cause5GMM.GetCauseValue()),
	}

	if msg.AuthenticationFailureParameter != nil {
		logger.EllaLog.Warn("AuthenticationFailureParameter not yet implemented")
	}

	return authFailure
}

func buildAuthenticationRequest(msg *nasMessage.AuthenticationRequest) *AuthenticationRequest {
	if msg == nil {
		return nil
	}

	authenticationRequest := &AuthenticationRequest{
		ExtendedProtocolDiscriminator:        msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:  msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		AuthenticationRequestMessageIdentity: nas.MessageName(msg.AuthenticationRequestMessageIdentity.Octet),
		SpareHalfOctetAndNgksi:               msg.SpareHalfOctetAndNgksi.Octet,
		ABBA:                                 msg.ABBA.GetABBAContents(),
	}

	if msg.AuthenticationParameterRAND != nil {
		authenticationRequest.AuthenticationParameterRAND = msg.AuthenticationParameterRAND.GetRANDValue()
	}

	if msg.AuthenticationParameterAUTN != nil {
		authenticationRequest.AuthenticationParameterAUTN = msg.AuthenticationParameterAUTN.GetAUTN()
	}

	if msg.EAPMessage != nil {
		authenticationRequest.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return authenticationRequest
}

func buildRegistrationComplete(msg *nasMessage.RegistrationComplete) *RegistrationComplete {
	if msg == nil {
		return nil
	}

	regComplete := &RegistrationComplete{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		RegistrationCompleteMessageIdentity: nas.MessageName(msg.RegistrationCompleteMessageIdentity.Octet),
	}

	if msg.SORTransparentContainer != nil {
		regComplete.GetSORContent = msg.SORTransparentContainer.GetSORContent()
	}

	return regComplete
}

func buildRegistrationResult5GS(msg nasType.RegistrationResult5GS) string {
	value := msg.GetRegistrationResultValue5GS()
	switch {
	case value&(nasMessage.AccessType3GPP|nasMessage.AccessTypeNon3GPP) == (nasMessage.AccessType3GPP | nasMessage.AccessTypeNon3GPP):
		return "3GPP and Non-3GPP"
	case value&nasMessage.AccessType3GPP != 0:
		return "3GPP only"
	case value&nasMessage.AccessTypeNon3GPP != 0:
		return "Non-3GPP only"
	default:
		return fmt.Sprintf("Unknown(%d)", value)
	}
}

func buildRegistrationAccept(msg *nasMessage.RegistrationAccept) *RegistrationAccept {
	if msg == nil {
		return nil
	}

	registrationAccept := &RegistrationAccept{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		RegistrationAcceptMessageIdentity:   nas.MessageName(msg.RegistrationAcceptMessageIdentity.Octet),
		RegistrationResult5GS:               buildRegistrationResult5GS(msg.RegistrationResult5GS),
	}

	if msg.GUTI5G != nil {
		guti := buildGUTI5G(*msg.GUTI5G)
		registrationAccept.GUTI5G = &guti
	}

	if msg.EquivalentPlmns != nil {
		registrationAccept.EquivalentPLMNs = equivalentPlmnsToList(*msg.EquivalentPlmns)
	}

	if msg.TAIList != nil {
		taiList := nasToTaiList(msg.TAIList)
		registrationAccept.TAIList = taiList
	}

	if msg.AllowedNSSAI != nil {
		allowedNssai := buildNASAllowedSNSSAI(*msg.AllowedNSSAI)
		registrationAccept.AllowedNSSAI = allowedNssai
	}

	if msg.RejectedNSSAI != nil {
		logger.EllaLog.Warn("RejectedNSSAI not yet implemented")
	}

	if msg.ConfiguredNSSAI != nil {
		logger.EllaLog.Warn("ConfiguredNSSAI not yet implemented")
	}

	if msg.NetworkFeatureSupport5GS != nil {
		networkfeatureSupport5Gs := buildNetworkFeatureSupport5GS(*msg.NetworkFeatureSupport5GS)
		registrationAccept.NetworkFeatureSupport5GS = &networkfeatureSupport5Gs
	}

	if msg.PDUSessionStatus != nil {
		logger.EllaLog.Warn("PDUSessionStatus not yet implemented")
	}

	if msg.PDUSessionReactivationResult != nil {
		logger.EllaLog.Warn("PDUSessionReactivationResult not yet implemented")
	}

	if msg.PDUSessionReactivationResultErrorCause != nil {
		logger.EllaLog.Warn("PDUSessionReactivationResultErrorCause not yet implemented")
	}

	if msg.LADNInformation != nil {
		logger.EllaLog.Warn("LADNInformation not yet implemented")
	}

	if msg.MICOIndication != nil {
		logger.EllaLog.Warn("MICOIndication not yet implemented")
	}

	if msg.NetworkSlicingIndication != nil {
		logger.EllaLog.Warn("NetworkSlicingIndication not yet implemented")
	}

	if msg.ServiceAreaList != nil {
		logger.EllaLog.Warn("ServiceAreaList not yet implemented")
	}

	if msg.ServiceAreaList != nil {
		logger.EllaLog.Warn("ServiceAreaList not yet implemented")
	}

	if msg.T3512Value != nil {
		logger.EllaLog.Warn("T3512Value not yet implemented")
	}

	if msg.Non3GppDeregistrationTimerValue != nil {
		logger.EllaLog.Warn("Non3GppDeregistrationTimerValue not yet implemented")
	}
	if msg.T3502Value != nil {
		logger.EllaLog.Warn("T3502Value not yet implemented")
	}
	if msg.EmergencyNumberList != nil {
		logger.EllaLog.Warn("EmergencyNumberList not yet implemented")
	}
	if msg.ExtendedEmergencyNumberList != nil {
		logger.EllaLog.Warn("ExtendedEmergencyNumberList not yet implemented")
	}
	if msg.SORTransparentContainer != nil {
		logger.EllaLog.Warn("SORTransparentContainer not yet implemented")
	}

	if msg.EAPMessage != nil {
		logger.EllaLog.Warn("EAPMessage not yet implemented")
	}

	if msg.NSSAIInclusionMode != nil {
		logger.EllaLog.Warn("NSSAIInclusionMode not yet implemented")
	}

	if msg.OperatordefinedAccessCategoryDefinitions != nil {
		logger.EllaLog.Warn("OperatordefinedAccessCategoryDefinitions not yet implemented")
	}

	if msg.NegotiatedDRXParameters != nil {
		logger.EllaLog.Warn("NegotiatedDRXParameters not yet implemented")
	}

	return registrationAccept
}

func buildNetworkFeatureSupport5GS(msg nasType.NetworkFeatureSupport5GS) NetworkFeatureSupport5GS {
	return NetworkFeatureSupport5GS{
		Emc:          msg.GetEMC(),
		EmcN3:        msg.GetEMCN(),
		Emf:          msg.GetEMF(),
		IwkN26:       msg.GetIWKN26(),
		Mpsi:         msg.GetMPSI(),
		Mcsi:         msg.GetMCSI(),
		IMSVoPS3GPP:  msg.GetIMSVoPS3GPP(),
		IMSVoPSN3GPP: msg.GetIMSVoPSN3GPP(),
	}
}

func plmnFromNas3(b0, b1, b2 uint8) (PLMNID, error) {
	mcc1 := int(b0 & 0x0F)
	mcc2 := int((b0 >> 4) & 0x0F)
	mcc3 := int(b1 & 0x0F)
	mnc3 := int((b1 >> 4) & 0x0F)
	mnc1 := int(b2 & 0x0F)
	mnc2 := int((b2 >> 4) & 0x0F)

	// basic digit checks
	if mcc1 > 9 || mcc2 > 9 || mcc3 > 9 || mnc1 > 9 || mnc2 > 9 || (mnc3 != 0xF && mnc3 > 9) {
		return PLMNID{}, fmt.Errorf("invalid BCD digits in PLMN: %02x %02x %02x", b0, b1, b2)
	}

	plmn := PLMNID{
		Mcc: fmt.Sprintf("%d%d%d", mcc1, mcc2, mcc3),
	}
	if mnc3 == 0xF {
		plmn.Mnc = fmt.Sprintf("%d%d", mnc1, mnc2) // 2-digit MNC
	} else {
		plmn.Mnc = fmt.Sprintf("%d%d%d", mnc1, mnc2, mnc3) // 3-digit MNC
	}
	return plmn, nil
}

func buildNASAllowedSNSSAI(msg nasType.AllowedNSSAI) []SNSSAI {
	value := msg.GetSNSSAIValue()
	out := make([]SNSSAI, 0, 4)

	for i := 0; i < len(value); {
		if i >= len(value) {
			logger.EllaLog.Warn("AllowedNSSAI: unexpected end of buffer")
			break
		}
		l := int(value[i])
		i++

		if l != 1 && l != 4 {
			logger.EllaLog.Warn("AllowedNSSAI: unsupported or malformed element length", zap.Int("length", l))
			break
		}
		if i+l > len(value) {
			logger.EllaLog.Warn("AllowedNSSAI: element length exceeds buffer", zap.Int("length", l), zap.Int("remaining", len(value)-i))
			break
		}

		sst := int32(value[i])
		if l == 1 {
			out = append(out, SNSSAI{
				SST: sst,
				SD:  nil,
			})
			i += 1
			continue
		}

		// l == 4 → SST + 3-byte SD
		sdBytes := value[i+1 : i+4]
		sdStr := hex.EncodeToString(sdBytes)
		out = append(out, SNSSAI{
			SST: sst,
			SD:  &sdStr,
		})
		i += 4
	}

	return out
}

// nasToTaiList decodes the NAS-encoded TAI list produced by TaiListToNas.
func nasToTaiList(nas *nasType.TAIList) []TAI {
	if nas == nil {
		return nil
	}

	data := nas.GetPartialTrackingAreaIdentityList()

	if len(data) < 1 {
		logger.EllaLog.Warn("TAIList too short")
		return nil
	}

	header := data[0]
	typeOfList := int((header >> 5) & 0x07) // top 3 bits
	n := int(header&0x1F) + 1               // number of TAIs

	switch typeOfList {
	case 0x00:
		// Structure: [HDR][PLMN(3)][TAC(3) x N]
		minLen := 1 + 3 + 3*n
		if len(data) < minLen {
			return nil
		}
		idx := 1
		plmn, err := plmnFromNas3(data[idx], data[idx+1], data[idx+2])
		if err != nil {
			return nil
		}
		idx += 3

		out := make([]TAI, 0, n)
		for range n {
			tacBytes := data[idx : idx+3]
			idx += 3
			out = append(out, TAI{
				PLMNID: plmn,                         // same PLMN for all
				TAC:    hex.EncodeToString(tacBytes), // 6 hex chars
			})
		}

		if idx != len(data) {
			logger.EllaLog.Warn("TAIList has trailing bytes")
		}
		return out

	case 0x02:
		// Structure: [HDR][PLMN(3)+TAC(3)] x N
		minLen := 1 + n*6
		if len(data) < minLen {
			return nil
		}
		idx := 1
		out := make([]TAI, 0, n)
		for range n {
			plmn, err := plmnFromNas3(data[idx], data[idx+1], data[idx+2])
			if err != nil {
				logger.EllaLog.Warn("TAIList invalid PLMN", zap.Error(err))
				return nil
			}
			idx += 3
			tacBytes := data[idx : idx+3]
			idx += 3
			out = append(out, TAI{
				PLMNID: plmn,
				TAC:    hex.EncodeToString(tacBytes),
			})
		}
		if idx != len(data) {
			logger.EllaLog.Warn("TAIList has trailing bytes")
		}
		return out

	default:
		return nil
	}
}

func buildGUTI5G(gutiNas nasType.GUTI5G) string {
	mcc1 := gutiNas.GetMCCDigit1()
	mcc2 := gutiNas.GetMCCDigit2()
	mcc3 := gutiNas.GetMCCDigit3()
	mnc1 := gutiNas.GetMNCDigit1()
	mnc2 := gutiNas.GetMNCDigit2()
	mnc3 := gutiNas.GetMNCDigit3()

	amfRegionID := gutiNas.GetAMFRegionID()
	amfSetID := gutiNas.GetAMFSetID()
	amfPointer := gutiNas.GetAMFPointer()
	amfID := NasToAmfId(amfRegionID, amfSetID, amfPointer)

	tmsi := hex.EncodeToString(gutiNas.Octet[7:11])

	if mnc3 == 0x0F {
		return fmt.Sprintf("%d%d%d%d%d%s%s", mcc1, mcc2, mcc3, mnc1, mnc2, amfID, tmsi)
	}

	return fmt.Sprintf("%d%d%d%d%d%d%s%s", mcc1, mcc2, mcc3, mnc1, mnc2, mnc3, amfID, tmsi)
}

// func buildEquivalentPLMNs(msg nasType.EquivalentPlmns) []PLMNID {
// 	equivalentPLMNs := []PLMNID{}
// 	return equivalentPLMNs
// }

func nasPlmn3ToID(b0, b1, b2 uint8) (PLMNID, error) {
	mcc1 := int(b0 & 0x0F)
	mcc2 := int((b0 >> 4) & 0x0F)
	mcc3 := int(b1 & 0x0F)
	mnc3 := int((b1 >> 4) & 0x0F)
	mnc1 := int(b2 & 0x0F)
	mnc2 := int((b2 >> 4) & 0x0F)

	// Basic digit validation (0..9 or 0xF for mnc3)
	if mcc1 > 9 || mcc2 > 9 || mcc3 > 9 || mnc1 > 9 || mnc2 > 9 || (mnc3 != 0x0F && mnc3 > 9) {
		return PLMNID{}, fmt.Errorf("invalid BCD digits in PLMN bytes: %02x %02x %02x", b0, b1, b2)
	}

	mcc := fmt.Sprintf("%d%d%d", mcc1, mcc2, mcc3)
	var mnc string
	if mnc3 == 0x0F {
		// 2-digit MNC
		mnc = fmt.Sprintf("%d%d", mnc1, mnc2)
	} else {
		// 3-digit MNC
		mnc = fmt.Sprintf("%d%d%d", mnc1, mnc2, mnc3)
	}

	return PLMNID{Mcc: mcc, Mnc: mnc}, nil
}

// Full inverse for the NAS Equivalent PLMNs IE.
// EquivalentPlmns.Len is the number of bytes in Octet actually used (multiple of 3).
func equivalentPlmnsToList(eq nasType.EquivalentPlmns) []PLMNID {
	if eq.Len == 0 {
		logger.EllaLog.Warn("EquivalentPlmns length is zero")
		return nil
	}

	if eq.Len%3 != 0 {
		logger.EllaLog.Warn("EquivalentPlmns length not multiple of 3")
		return nil
	}

	if int(eq.Len) > len(eq.Octet) {
		logger.EllaLog.Warn("EquivalentPlmns has trailing bytes")
		return nil
	}

	n := int(eq.Len) / 3
	out := make([]PLMNID, 0, n)

	for i := range n {
		base := i * 3
		plmn, err := nasPlmn3ToID(eq.Octet[base], eq.Octet[base+1], eq.Octet[base+2])
		if err != nil {
			logger.EllaLog.Warn("EquivalentPlmns invalid PLMN", zap.Error(err))
			return nil
		}
		out = append(out, plmn)
	}

	return out
}

func NasToAmfId(regionID uint8, setID uint16, pointer uint8) string {
	setID &= 0x03FF // 10 bits
	pointer &= 0x3F // 6 bits

	b0 := regionID
	b1 := uint8(setID >> 2)
	b2 := uint8((setID&0x3)<<6) | (pointer & 0x3F)

	return fmt.Sprintf("%02x%02x%02x", b0, b1, b2)
}

func buildRegistrationRequest(msg *nasMessage.RegistrationRequest) *RegistrationRequest {
	if msg == nil {
		return nil
	}

	registrationRequest := &RegistrationRequest{
		MobileIdentity5GS:                  getMobileIdentity5GS(msg.MobileIdentity5GS),
		ExtendedProtocolDiscriminator:      msg.ExtendedProtocolDiscriminator.Octet,
		NgksiAndRegistrationType5GS:        msg.NgksiAndRegistrationType5GS.Octet,
		RegistrationRequestMessageIdentity: nas.MessageName(msg.RegistrationRequestMessageIdentity.Octet),
	}

	if msg.NoncurrentNativeNASKeySetIdentifier != nil {
		logger.EllaLog.Warn("NoncurrentNativeNASKeySetIdentifier not yet implemented")
	}

	if msg.Capability5GMM != nil {
		logger.EllaLog.Warn("Capability5GMM not yet implemented")
	}

	if msg.UESecurityCapability != nil {
		registrationRequest.UESecurityCapability = buildUESecurityCapability(*msg.UESecurityCapability)
	}

	if msg.RequestedNSSAI != nil {
		logger.EllaLog.Warn("RequestedNSSAI not yet implemented")
	}

	if msg.LastVisitedRegisteredTAI != nil {
		logger.EllaLog.Warn("LastVisitedRegisteredTAI not yet implemented")
	}

	if msg.S1UENetworkCapability != nil {
		logger.EllaLog.Warn("S1UENetworkCapability not yet implemented")
	}

	if msg.UplinkDataStatus != nil {
		logger.EllaLog.Warn("UplinkDataStatus not yet implemented")
	}

	if msg.PDUSessionStatus != nil {
		logger.EllaLog.Warn("PDUSessionStatus not yet implemented")
	}

	if msg.MICOIndication != nil {
		logger.EllaLog.Warn("MICOIndication not yet implemented")
	}

	if msg.UEStatus != nil {
		logger.EllaLog.Warn("UEStatus not yet implemented")
	}

	if msg.AdditionalGUTI != nil {
		logger.EllaLog.Warn("AdditionalGUTI not yet implemented")
	}

	if msg.AllowedPDUSessionStatus != nil {
		logger.EllaLog.Warn("AllowedPDUSessionStatus not yet implemented")
	}

	if msg.UesUsageSetting != nil {
		logger.EllaLog.Warn("UesUsageSetting not yet implemented")
	}

	if msg.RequestedDRXParameters != nil {
		logger.EllaLog.Warn("RequestedDRXParameters not yet implemented")
	}

	if msg.EPSNASMessageContainer != nil {
		logger.EllaLog.Warn("EPSNASMessageContainer not yet implemented")
	}

	if msg.LADNIndication != nil {
		logger.EllaLog.Warn("LADNIndication not yet implemented")
	}

	if msg.PayloadContainer != nil {
		logger.EllaLog.Warn("PayloadContainer not yet implemented")
	}

	if msg.NetworkSlicingIndication != nil {
		logger.EllaLog.Warn("NetworkSlicingIndication not yet implemented")
	}

	if msg.UpdateType5GS != nil {
		logger.EllaLog.Warn("UpdateType5GS not yet implemented")
	}

	if msg.NASMessageContainer != nil {
		logger.EllaLog.Warn("NASMessageContainer not yet implemented")
	}

	return registrationRequest
}

func buildReplayedUESecurityCapability(ueSecurityCapability nasType.ReplayedUESecurityCapabilities) *UESecurityCapability {
	ueSecCap := &UESecurityCapability{
		IntegrityAlgorithm: IntegrityAlgorithm{},
		CipheringAlgorithm: CipheringAlgorithm{},
	}

	if ueSecurityCapability.GetIA0_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA0 = true
	}

	if ueSecurityCapability.GetIA1_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA1 = true
	}

	if ueSecurityCapability.GetIA2_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA2 = true
	}

	if ueSecurityCapability.GetIA3_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA3 = true
	}

	if ueSecurityCapability.GetEA0_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA0 = true
	}

	if ueSecurityCapability.GetEA1_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA1 = true
	}

	if ueSecurityCapability.GetEA2_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA2 = true
	}

	if ueSecurityCapability.GetEA3_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA3 = true
	}

	return ueSecCap
}

func buildUESecurityCapability(ueSecurityCapability nasType.UESecurityCapability) *UESecurityCapability {
	ueSecCap := &UESecurityCapability{
		IntegrityAlgorithm: IntegrityAlgorithm{},
		CipheringAlgorithm: CipheringAlgorithm{},
	}

	if ueSecurityCapability.GetIA0_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA0 = true
	}

	if ueSecurityCapability.GetIA1_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA1 = true
	}

	if ueSecurityCapability.GetIA2_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA2 = true
	}

	if ueSecurityCapability.GetIA3_128_5G() == 1 {
		ueSecCap.IntegrityAlgorithm.NIA3 = true
	}

	if ueSecurityCapability.GetEA0_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA0 = true
	}

	if ueSecurityCapability.GetEA1_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA1 = true
	}

	if ueSecurityCapability.GetEA2_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA2 = true
	}

	if ueSecurityCapability.GetEA3_128_5G() == 1 {
		ueSecCap.CipheringAlgorithm.NEA3 = true
	}

	return ueSecCap
}

func getMobileIdentity5GS(mobileIdentity5GS nasType.MobileIdentity5GS) MobileIdentity5GS {
	mobileIdentity5GSContents := mobileIdentity5GS.GetMobileIdentity5GSContents()
	identityTypeUsedForRegistration := nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch identityTypeUsedForRegistration {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		return MobileIdentity5GS{
			Identity: "No Identity",
		}
	case nasMessage.MobileIdentity5GSTypeSuci:
		suci, plmnID := nasConvert.SuciToString(mobileIdentity5GSContents)
		plmnIDModel := PlmnIDStringToModels(plmnID)
		return MobileIdentity5GS{
			Identity: "SUCI",
			SUCI:     &suci,
			PLMNID:   &plmnIDModel,
		}
	case nasMessage.MobileIdentity5GSType5gGuti:
		_, guti := util.GutiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			GUTI:     &guti,
			Identity: "5G-GUTI",
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			Identity: "IMEI",
			IMEI:     &imei,
		}
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		sTmsi := hex.EncodeToString(mobileIdentity5GSContents[1:])
		return MobileIdentity5GS{
			STMSI:    &sTmsi,
			Identity: "5G-S-TMSI",
		}
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		return MobileIdentity5GS{
			Identity: "IMEISV",
			IMEISV:   &imeisv,
		}
	default:
		logger.EllaLog.Warn("MobileIdentity5GS type not fully implemented", zap.String("identity_type", fmt.Sprintf("%v", identityTypeUsedForRegistration)))
		return MobileIdentity5GS{
			Identity: "Unknown",
		}
	}
}

func PlmnIDStringToModels(plmnIDStr string) PLMNID {
	var plmnID PLMNID
	plmnID.Mcc = plmnIDStr[:3]
	plmnID.Mnc = plmnIDStr[3:]
	return plmnID
}

func buildGsmMessage(msg *nas.GsmMessage) *GsmMessage {
	if msg == nil {
		return nil
	}

	return &GsmMessage{
		GsmHeader: GsmHeader{
			MessageType: getGsmMessageType(msg),
		},
	}
}

func getGsmMessageType(msg *nas.GsmMessage) string {
	switch msg.GetMessageType() {
	case nas.MsgTypePDUSessionEstablishmentRequest:
		return fmt.Sprintf("PDUSessionEstablishmentRequest (%v)", nas.MsgTypePDUSessionEstablishmentRequest)
	case nas.MsgTypePDUSessionEstablishmentAccept:
		return fmt.Sprintf("PDUSessionEstablishmentAccept (%v)", nas.MsgTypePDUSessionEstablishmentAccept)
	case nas.MsgTypePDUSessionEstablishmentReject:
		return fmt.Sprintf("PDUSessionEstablishmentReject (%v)", nas.MsgTypePDUSessionEstablishmentReject)
	case nas.MsgTypePDUSessionAuthenticationCommand:
		return fmt.Sprintf("PDUSessionAuthenticationCommand (%v)", nas.MsgTypePDUSessionAuthenticationCommand)
	case nas.MsgTypePDUSessionAuthenticationComplete:
		return fmt.Sprintf("PDUSessionAuthenticationComplete (%v)", nas.MsgTypePDUSessionAuthenticationComplete)
	case nas.MsgTypePDUSessionAuthenticationResult:
		return fmt.Sprintf("PDUSessionAuthenticationResult (%v)", nas.MsgTypePDUSessionAuthenticationResult)
	case nas.MsgTypePDUSessionModificationRequest:
		return fmt.Sprintf("PDUSessionModificationRequest (%v)", nas.MsgTypePDUSessionModificationRequest)
	case nas.MsgTypePDUSessionModificationReject:
		return fmt.Sprintf("PDUSessionModificationReject (%v)", nas.MsgTypePDUSessionModificationReject)
	case nas.MsgTypePDUSessionModificationCommand:
		return fmt.Sprintf("PDUSessionModificationCommand (%v)", nas.MsgTypePDUSessionModificationCommand)
	case nas.MsgTypePDUSessionModificationComplete:
		return fmt.Sprintf("PDUSessionModificationComplete (%v)", nas.MsgTypePDUSessionModificationComplete)
	case nas.MsgTypePDUSessionModificationCommandReject:
		return fmt.Sprintf("PDUSessionModificationCommandReject (%v)", nas.MsgTypePDUSessionModificationCommandReject)
	case nas.MsgTypePDUSessionReleaseRequest:
		return fmt.Sprintf("PDUSessionReleaseRequest (%v)", nas.MsgTypePDUSessionReleaseRequest)
	case nas.MsgTypePDUSessionReleaseReject:
		return fmt.Sprintf("PDUSessionReleaseReject (%v)", nas.MsgTypePDUSessionReleaseReject)
	case nas.MsgTypePDUSessionReleaseCommand:
		return fmt.Sprintf("PDUSessionReleaseCommand (%v)", nas.MsgTypePDUSessionReleaseCommand)
	case nas.MsgTypePDUSessionReleaseComplete:
		return fmt.Sprintf("PDUSessionReleaseComplete (%v)", nas.MsgTypePDUSessionReleaseComplete)
	case nas.MsgTypeStatus5GSM:
		return fmt.Sprintf("Status5GSM (%v)", nas.MsgTypeStatus5GSM)
	default:
		return fmt.Sprintf("Unknown (%v)", msg.GetMessageType())
	}
}

func getGmmMessageType(msg *nas.GmmMessage) string {
	switch msg.GetMessageType() {
	case nas.MsgTypeRegistrationRequest:
		return fmt.Sprintf("RegistrationRequest (%v)", nas.MsgTypeRegistrationRequest)
	case nas.MsgTypeRegistrationAccept:
		return fmt.Sprintf("RegistrationAccept (%v)", nas.MsgTypeRegistrationAccept)
	case nas.MsgTypeRegistrationComplete:
		return fmt.Sprintf("RegistrationComplete (%v)", nas.MsgTypeRegistrationComplete)
	case nas.MsgTypeRegistrationReject:
		return fmt.Sprintf("RegistrationReject (%v)", nas.MsgTypeRegistrationReject)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return fmt.Sprintf("DeregistrationRequestUEOriginatingDeregistration (%v)", nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)
	case nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:
		return fmt.Sprintf("DeregistrationAcceptUEOriginatingDeregistration (%v)", nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration)
	case nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:
		return fmt.Sprintf("DeregistrationRequestUETerminatedDeregistration (%v)", nas.MsgTypeDeregistrationRequestUETerminatedDeregistration)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return fmt.Sprintf("DeregistrationAcceptUETerminatedDeregistration (%v)", nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration)
	case nas.MsgTypeServiceRequest:
		return fmt.Sprintf("ServiceRequest (%v)", nas.MsgTypeServiceRequest)
	case nas.MsgTypeServiceReject:
		return fmt.Sprintf("ServiceReject (%v)", nas.MsgTypeServiceReject)
	case nas.MsgTypeServiceAccept:
		return fmt.Sprintf("ServiceAccept (%v)", nas.MsgTypeServiceAccept)
	case nas.MsgTypeConfigurationUpdateCommand:
		return fmt.Sprintf("ConfigurationUpdateCommand (%v)", nas.MsgTypeConfigurationUpdateCommand)
	case nas.MsgTypeConfigurationUpdateComplete:
		return fmt.Sprintf("ConfigurationUpdateComplete (%v)", nas.MsgTypeConfigurationUpdateComplete)
	case nas.MsgTypeAuthenticationRequest:
		return fmt.Sprintf("AuthenticationRequest (%v)", nas.MsgTypeAuthenticationRequest)
	case nas.MsgTypeAuthenticationResponse:
		return fmt.Sprintf("AuthenticationResponse (%v)", nas.MsgTypeAuthenticationResponse)
	case nas.MsgTypeAuthenticationReject:
		return fmt.Sprintf("AuthenticationReject (%v)", nas.MsgTypeAuthenticationReject)
	case nas.MsgTypeAuthenticationFailure:
		return fmt.Sprintf("AuthenticationFailure (%v)", nas.MsgTypeAuthenticationFailure)
	case nas.MsgTypeAuthenticationResult:
		return fmt.Sprintf("AuthenticationResult (%v)", nas.MsgTypeAuthenticationResult)
	case nas.MsgTypeIdentityRequest:
		return fmt.Sprintf("IdentityRequest (%v)", nas.MsgTypeIdentityRequest)
	case nas.MsgTypeIdentityResponse:
		return fmt.Sprintf("IdentityResponse (%v)", nas.MsgTypeIdentityResponse)
	case nas.MsgTypeSecurityModeCommand:
		return fmt.Sprintf("SecurityModeCommand (%v)", nas.MsgTypeSecurityModeCommand)
	case nas.MsgTypeSecurityModeComplete:
		return fmt.Sprintf("SecurityModeComplete (%v)", nas.MsgTypeSecurityModeComplete)
	case nas.MsgTypeSecurityModeReject:
		return fmt.Sprintf("SecurityModeReject (%v)", nas.MsgTypeSecurityModeReject)
	case nas.MsgTypeStatus5GMM:
		return fmt.Sprintf("Status5GMM (%v)", nas.MsgTypeStatus5GMM)
	case nas.MsgTypeNotification:
		return fmt.Sprintf("Notification (%v)", nas.MsgTypeNotification)
	case nas.MsgTypeNotificationResponse:
		return fmt.Sprintf("NotificationResponse (%v)", nas.MsgTypeNotificationResponse)
	case nas.MsgTypeULNASTransport:
		return fmt.Sprintf("ULNASTransport (%v)", nas.MsgTypeULNASTransport)
	case nas.MsgTypeDLNASTransport:
		return fmt.Sprintf("DLNASTransport (%v)", nas.MsgTypeDLNASTransport)
	default:
		return fmt.Sprintf("Unknown (%v)", msg.GetMessageType())
	}
}

func decodeNAS(raw []byte, nasContextInfo *NasContextInfo) (*nas.Message, error) {
	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(raw) & 0x0f

	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypePlainNas:
		if err := msg.PlainNasDecode(&raw); err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	case nas.SecurityHeaderTypeIntegrityProtected:
		p := raw[7:]
		if err := msg.PlainNasDecode(&p); err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		if nasContextInfo == nil {
			return nil, fmt.Errorf("nas context info is nil")
		}

		amf := context.AMFSelf()

		ranUE := amf.RanUeFindByAmfUeNgapID(nasContextInfo.AMFUENGAPID)
		if ranUE == nil {
			return nil, fmt.Errorf("ran ue is nil")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("amf ue is nil")
		}

		decrypted, err := DecryptNASMessage(ranUE.AmfUe, nasContextInfo.Direction, raw)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt NAS message: %w", err)
		}

		err = msg.PlainNasDecode(&decrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		if nasContextInfo == nil {
			return nil, fmt.Errorf("nas context info is nil")
		}

		amf := context.AMFSelf()

		ranUE := amf.RanUeFindByAmfUeNgapID(nasContextInfo.AMFUENGAPID)
		if ranUE == nil {
			return nil, fmt.Errorf("ran ue is nil")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("amf ue is nil")
		}

		decrypted, err := DecryptNASMessage(ranUE.AmfUe, nasContextInfo.Direction, raw)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt NAS message: %w", err)
		}

		err = msg.PlainNasDecode(&decrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		if nasContextInfo == nil {
			return nil, fmt.Errorf("nas context info is nil")
		}

		amf := context.AMFSelf()

		ranUE := amf.RanUeFindByAmfUeNgapID(nasContextInfo.AMFUENGAPID)
		if ranUE == nil {
			return nil, fmt.Errorf("ran ue is nil")
		}

		if ranUE.AmfUe == nil {
			return nil, fmt.Errorf("amf ue is nil")
		}

		decrypted, err := DecryptNASMessage(ranUE.AmfUe, nasContextInfo.Direction, raw)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt NAS message: %w", err)
		}

		err = msg.PlainNasDecode(&decrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decode NAS message: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported security header type: %d", msg.SecurityHeaderType)
	}

	return msg, nil
}

func buildSecurityHeader(msg *nas.Message) SecurityHeader {
	securityHeaderType := ""
	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypePlainNas:
		securityHeaderType = "Plain NAS"
	case nas.SecurityHeaderTypeIntegrityProtected:
		securityHeaderType = "Integrity Protected"
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		securityHeaderType = "Integrity Protected and Ciphered"
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		securityHeaderType = "Integrity Protected with New 5G NAS Security Context"
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		securityHeaderType = "Integrity Protected and Ciphered with New 5G NAS Security Context"
	default:
		securityHeaderType = "Unknown"
	}

	return SecurityHeader{
		ProtocolDiscriminator:     msg.ProtocolDiscriminator,
		SecurityHeaderType:        securityHeaderType,
		MessageAuthenticationCode: msg.MessageAuthenticationCode,
		SequenceNumber:            msg.SequenceNumber,
	}
}
