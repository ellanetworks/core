package nas

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"go.uber.org/zap"
)

type AMBR struct {
	Value uint64 `json:"value"`
	Unit  string `json:"unit"`
}

type SessionAMBR struct {
	Uplink   AMBR `json:"uplink"`
	Downlink AMBR `json:"downlink"`
}

type PDUSessionEstablishmentAccept struct {
	ExtendedProtocolDiscriminator                uint8                                 `json:"extended_protocol_discriminator"`
	PDUSessionID                                 uint8                                 `json:"pdu_session_id"`
	PTI                                          uint8                                 `json:"pti"`
	PDUSESSIONESTABLISHMENTACCEPTMessageIdentity uint8                                 `json:"pdu_session_establishment_accept_message_identity"`
	SelectedSSCMode                              uint8                                 `json:"selected_ssc_mode"`
	SelectedPDUSessionType                       utils.EnumField[uint8]                `json:"selected_pdu_session_type"`
	AuthorizedQosRules                           []QosRule                             `json:"authorized_qos_rules"`
	SessionAMBR                                  SessionAMBR                           `json:"session_ambr"`
	Cause5GSM                                    *utils.EnumField[uint8]               `json:"cause_5g_s_m,omitempty"`
	PDUAddress                                   *string                               `json:"pdu_address,omitempty"`
	SNSSAI                                       *SNSSAI                               `json:"snssai,omitempty"`
	AuthorizedQosFlowDescriptions                []QoSFlowDescription                  `json:"authorized_qos_flow_descriptions,omitempty"`
	ExtendedProtocolConfigurationOptions         *ExtendedProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	DNN                                          *string                               `json:"dnn,omitempty"`

	RQTimerValue                 *UnsupportedIE `json:"rq_timer_value,omitempty"`
	AlwaysonPDUSessionIndication *UnsupportedIE `json:"alwayson_pdu_session_indication,omitempty"`
	MappedEPSBearerContexts      *UnsupportedIE `json:"mapped_eps_bearer_contexts,omitempty"`
	EAPMessage                   *UnsupportedIE `json:"eap_message,omitempty"`
}

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
		AuthorizedQosRules:     buildAuthorizedQosRules(msg.AuthorizedQosRules),
		SessionAMBR:            buildSessionAMBR(msg.SessionAMBR),
	}

	if msg.Cause5GSM != nil {
		cause := cause5GSMToString(msg.Cause5GSM.GetCauseValue())
		estAcc.Cause5GSM = &cause
	}

	if msg.PDUAddress != nil {
		address := buildPDUAddress(msg)
		estAcc.PDUAddress = &address
	}

	if msg.RQTimerValue != nil {
		estAcc.RQTimerValue = makeUnsupportedIE()
	}

	if msg.SNSSAI != nil {
		snssai := buildNSSAI(msg.SNSSAI)
		estAcc.SNSSAI = &snssai
	}

	if msg.AlwaysonPDUSessionIndication != nil {
		estAcc.AlwaysonPDUSessionIndication = makeUnsupportedIE()
	}

	if msg.MappedEPSBearerContexts != nil {
		estAcc.MappedEPSBearerContexts = makeUnsupportedIE()
	}

	if msg.EAPMessage != nil {
		estAcc.EAPMessage = makeUnsupportedIE()
	}

	if msg.AuthorizedQosFlowDescriptions != nil {
		estAcc.AuthorizedQosFlowDescriptions = buildAuthorizedQosFlowDescriptions(msg.AuthorizedQosFlowDescriptions)
	}

	if msg.ExtendedProtocolConfigurationOptions != nil {
		estAcc.ExtendedProtocolConfigurationOptions = buildExtendedProtocolConfigurationOptions(msg.ExtendedProtocolConfigurationOptions)
	}

	if msg.DNN != nil {
		dnn := string(msg.DNN.GetDNN())
		estAcc.DNN = &dnn
	}

	return estAcc
}

func buildAuthorizedQosFlowDescriptions(desc *nasType.AuthorizedQosFlowDescriptions) []QoSFlowDescription {
	if desc == nil {
		return nil
	}

	data := desc.GetQoSFlowDescriptions()

	flowDesc, err := ParseAuthorizedQosFlowDescriptions(data)
	if err != nil {
		logger.EllaLog.Warn("failed to parse AuthorizedQosFlowDescriptions", zap.Error(err))
		return nil
	}

	return flowDesc
}

func buildPDUSessionType(sessType uint8) utils.EnumField[uint8] {
	switch sessType {
	case nasMessage.PDUSessionTypeIPv4:
		return utils.MakeEnum(sessType, "IPv4", false)
	case nasMessage.PDUSessionTypeIPv6:
		return utils.MakeEnum(sessType, "IPv6", false)
	case nasMessage.PDUSessionTypeIPv4IPv6:
		return utils.MakeEnum(sessType, "IPv4v6", false)
	case nasMessage.PDUSessionTypeUnstructured:
		return utils.MakeEnum(sessType, "Unstructured", false)
	case nasMessage.PDUSessionTypeEthernet:
		return utils.MakeEnum(sessType, "Ethernet", false)
	default:
		return utils.MakeEnum(sessType, "", true)
	}
}

func buildAuthorizedQosRules(rules nasType.AuthorizedQosRules) []QosRule {
	qosRulesBytes := rules.GetQosRule()

	qosRules, err := UnmarshalQosRules(qosRulesBytes)
	if err != nil {
		logger.EllaLog.Warn("failed to unmarshal authorized QoS rules", zap.Error(err))
		return nil
	}

	return qosRules
}

func buildPDUAddress(msg *nasMessage.PDUSessionEstablishmentAccept) string {
	if msg.PDUAddress == nil {
		return ""
	}

	address := msg.GetPDUAddressInformation()

	pduAddr := net.IPv4(address[0], address[1], address[2], address[3])

	return pduAddr.String()
}

func buildSessionAMBR(ambr nasType.SessionAMBR) SessionAMBR {
	uplink := ambr.GetSessionAMBRForUplink()
	downlink := ambr.GetSessionAMBRForDownlink()

	uplinkUint64 := uint64(uplink[0])<<8 | uint64(uplink[1])
	downlinkUint64 := uint64(downlink[0])<<8 | uint64(downlink[1])

	uplinkUnit := ambr.GetUnitForSessionAMBRForDownlink()
	uplinkUnitStr := ambrUnitToString(uplinkUnit)

	downlinkUnit := ambr.GetUnitForSessionAMBRForDownlink()
	downlinkUnitStr := ambrUnitToString(downlinkUnit)

	return SessionAMBR{
		Uplink:   AMBR{Value: uplinkUint64, Unit: uplinkUnitStr},
		Downlink: AMBR{Value: downlinkUint64, Unit: downlinkUnitStr},
	}
}

func ambrUnitToString(unit uint8) string {
	switch unit {
	case nasMessage.SessionAMBRUnitNotUsed:
		return "bps"
	case nasMessage.SessionAMBRUnit1Kbps:
		return "Kbps"
	case nasMessage.SessionAMBRUnit1Mbps:
		return "Mbps"
	case nasMessage.SessionAMBRUnit1Gbps:
		return "Gbps"
	case nasMessage.SessionAMBRUnit1Tbps:
		return "Tbps"
	case nasMessage.SessionAMBRUnit1Pbps:
		return "Pbps"
	default:
		return fmt.Sprintf("Unknown(%d)", unit)
	}
}

func cause5GSMToString(causeValue uint8) utils.EnumField[uint8] {
	switch causeValue {
	case nasMessage.Cause5GSMInsufficientResources:
		return utils.MakeEnum(causeValue, "Insufficient Resources", false)
	case nasMessage.Cause5GSMMissingOrUnknownDNN:
		return utils.MakeEnum(causeValue, "Missing Or Unknown DNN", false)
	case nasMessage.Cause5GSMUnknownPDUSessionType:
		return utils.MakeEnum(causeValue, "Unknown PDU Session Type", false)
	case nasMessage.Cause5GSMUserAuthenticationOrAuthorizationFailed:
		return utils.MakeEnum(causeValue, "User Authentication Or Authorization Failed", false)
	case nasMessage.Cause5GSMRequestRejectedUnspecified:
		return utils.MakeEnum(causeValue, "Request Rejected Unspecified", false)
	case nasMessage.Cause5GSMServiceOptionTemporarilyOutOfOrder:
		return utils.MakeEnum(causeValue, "Service Option Temporarily Out Of Order", false)
	case nasMessage.Cause5GSMPTIAlreadyInUse:
		return utils.MakeEnum(causeValue, "PTI Already In Use", false)
	case nasMessage.Cause5GSMRegularDeactivation:
		return utils.MakeEnum(causeValue, "Regular Deactivation", false)
	case nasMessage.Cause5GSMReactivationRequested:
		return utils.MakeEnum(causeValue, "Reactivation Requested", false)
	case nasMessage.Cause5GSMInvalidPDUSessionIdentity:
		return utils.MakeEnum(causeValue, "Invalid PDU Session Identity", false)
	case nasMessage.Cause5GSMSemanticErrorsInPacketFilter:
		return utils.MakeEnum(causeValue, "Semantic Errors In Packet Filter", false)
	case nasMessage.Cause5GSMSyntacticalErrorInPacketFilter:
		return utils.MakeEnum(causeValue, "Syntactical Error In Packet Filter", false)
	case nasMessage.Cause5GSMOutOfLADNServiceArea:
		return utils.MakeEnum(causeValue, "Out Of LADN Service Area", false)
	case nasMessage.Cause5GSMPTIMismatch:
		return utils.MakeEnum(causeValue, "PTI Mismatch", false)
	case nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed:
		return utils.MakeEnum(causeValue, "PDU Session Type IPv4 Only Allowed", false)
	case nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed:
		return utils.MakeEnum(causeValue, "PDU Session Type IPv6 Only Allowed", false)
	case nasMessage.Cause5GSMPDUSessionDoesNotExist:
		return utils.MakeEnum(causeValue, "PDU Session Does Not Exist", false)
	case nasMessage.Cause5GSMInsufficientResourcesForSpecificSliceAndDNN:
		return utils.MakeEnum(causeValue, "Insufficient Resources For Specific Slice And DNN", false)
	case nasMessage.Cause5GSMNotSupportedSSCMode:
		return utils.MakeEnum(causeValue, "Not Supported SSC Mode", false)
	case nasMessage.Cause5GSMInsufficientResourcesForSpecificSlice:
		return utils.MakeEnum(causeValue, "Insufficient Resources For Specific Slice", false)
	case nasMessage.Cause5GSMMissingOrUnknownDNNInASlice:
		return utils.MakeEnum(causeValue, "Missing Or Unknown DNN In A Slice", false)
	case nasMessage.Cause5GSMInvalidPTIValue:
		return utils.MakeEnum(causeValue, "Invalid PTI Value", false)
	case nasMessage.Cause5GSMMaximumDataRatePerUEForUserPlaneIntegrityProtectionIsTooLow:
		return utils.MakeEnum(causeValue, "Maximum Data Rate Per UE For User Plane Integrity Protection Is Too Low", false)
	case nasMessage.Cause5GSMSemanticErrorInTheQoSOperation:
		return utils.MakeEnum(causeValue, "Semantic Error In The QoS Operation", false)
	case nasMessage.Cause5GSMSyntacticalErrorInTheQoSOperation:
		return utils.MakeEnum(causeValue, "Syntactical Error In The QoS Operation", false)
	case nasMessage.Cause5GSMInvalidMappedEPSBearerIdentity:
		return utils.MakeEnum(causeValue, "Invalid Mapped EPS Bearer Identity", false)
	case nasMessage.Cause5GSMSemanticallyIncorrectMessage:
		return utils.MakeEnum(causeValue, "Semantically Incorrect Message", false)
	case nasMessage.Cause5GSMInvalidMandatoryInformation:
		return utils.MakeEnum(causeValue, "Invalid Mandatory Information", false)
	case nasMessage.Cause5GSMMessageTypeNonExistentOrNotImplemented:
		return utils.MakeEnum(causeValue, "Message Type Non Existent Or Not Implemented", false)
	case nasMessage.Cause5GSMMessageTypeNotCompatibleWithTheProtocolState:
		return utils.MakeEnum(causeValue, "Message Type Not Compatible With The Protocol State", false)
	case nasMessage.Cause5GSMInformationElementNonExistentOrNotImplemented:
		return utils.MakeEnum(causeValue, "Information Element Non Existent Or Not Implemented", false)
	case nasMessage.Cause5GSMConditionalIEError:
		return utils.MakeEnum(causeValue, "Conditional IE Error", false)
	case nasMessage.Cause5GSMMessageNotCompatibleWithTheProtocolState:
		return utils.MakeEnum(causeValue, "Message Not Compatible With The Protocol State", false)
	case nasMessage.Cause5GSMProtocolErrorUnspecified:
		return utils.MakeEnum(causeValue, "Protocol Error Unspecified", false)
	default:
		return utils.MakeEnum(causeValue, "", true)
	}
}
