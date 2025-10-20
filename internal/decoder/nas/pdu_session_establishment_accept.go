package nas

import (
	"fmt"
	"net"

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
	SelectedPDUSessionType                       string                                `json:"selected_pdu_session_type"`
	AuthorizedQosRules                           []QosRule                             `json:"authorized_qos_rules"`
	SessionAMBR                                  SessionAMBR                           `json:"session_ambr"`
	Cause5GSM                                    *string                               `json:"cause_5g_s_m,omitempty"`
	PDUAddress                                   *string                               `json:"pdu_address,omitempty"`
	SNSSAI                                       *SNSSAI                               `json:"snssai,omitempty"`
	AuthorizedQosFlowDescriptions                []QoSFlowDescription                  `json:"authorized_qos_flow_descriptions,omitempty"`
	ExtendedProtocolConfigurationOptions         *ExtendedProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	DNN                                          *string                               `json:"dnn,omitempty"`
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
		logger.EllaLog.Warn("RQTimerValue not yet implemented")
	}

	if msg.SNSSAI != nil {
		snssai := buildNSSAI(msg.SNSSAI)
		estAcc.SNSSAI = &snssai
	}

	if msg.AlwaysonPDUSessionIndication != nil {
		logger.EllaLog.Warn("AlwaysonPDUSessionIndication not yet implemented")
	}

	if msg.MappedEPSBearerContexts != nil {
		logger.EllaLog.Warn("MappedEPSBearerContexts not yet implemented")
	}

	if msg.EAPMessage != nil {
		logger.EllaLog.Warn("EAPMessage not yet implemented")
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

func cause5GSMToString(causeValue uint8) string {
	switch causeValue {
	case nasMessage.Cause5GSMInsufficientResources:
		return "Insufficient Resources"
	case nasMessage.Cause5GSMMissingOrUnknownDNN:
		return "Missing Or Unknown DNN"
	case nasMessage.Cause5GSMUnknownPDUSessionType:
		return "Unknown PDU Session Type"
	case nasMessage.Cause5GSMUserAuthenticationOrAuthorizationFailed:
		return "User Authentication Or Authorization Failed"
	case nasMessage.Cause5GSMRequestRejectedUnspecified:
		return "Request Rejected Unspecified"
	case nasMessage.Cause5GSMServiceOptionTemporarilyOutOfOrder:
		return "Service Option Temporarily Out Of Order"
	case nasMessage.Cause5GSMPTIAlreadyInUse:
		return "PTI Already In Use"
	case nasMessage.Cause5GSMRegularDeactivation:
		return "Regular Deactivation"
	case nasMessage.Cause5GSMReactivationRequested:
		return "Reactivation Requested"
	case nasMessage.Cause5GSMInvalidPDUSessionIdentity:
		return "Invalid PDU Session Identity"
	case nasMessage.Cause5GSMSemanticErrorsInPacketFilter:
		return "Semantic Errors In Packet Filter"
	case nasMessage.Cause5GSMSyntacticalErrorInPacketFilter:
		return "Syntactical Error In Packet Filter"
	case nasMessage.Cause5GSMOutOfLADNServiceArea:
		return "Out Of LADN Service Area"
	case nasMessage.Cause5GSMPTIMismatch:
		return "PTI Mismatch"
	case nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed:
		return "PDU Session Type IPv4 Only Allowed"
	case nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed:
		return "PDU Session Type IPv6 Only Allowed"
	case nasMessage.Cause5GSMPDUSessionDoesNotExist:
		return "PDU Session Does Not Exist"
	case nasMessage.Cause5GSMInsufficientResourcesForSpecificSliceAndDNN:
		return "Insufficient Resources For Specific Slice And DNN"
	case nasMessage.Cause5GSMNotSupportedSSCMode:
		return "Not Supported SSC Mode"
	case nasMessage.Cause5GSMInsufficientResourcesForSpecificSlice:
		return "Insufficient Resources For Specific Slice"
	case nasMessage.Cause5GSMMissingOrUnknownDNNInASlice:
		return "Missing Or Unknown DNN In A Slice"
	case nasMessage.Cause5GSMInvalidPTIValue:
		return "Invalid PTI Value"
	case nasMessage.Cause5GSMMaximumDataRatePerUEForUserPlaneIntegrityProtectionIsTooLow:
		return "Maximum Data Rate Per UE For User Plane Integrity Protection Is Too Low"
	case nasMessage.Cause5GSMSemanticErrorInTheQoSOperation:
		return "Semantic Error In The QoS Operation"
	case nasMessage.Cause5GSMSyntacticalErrorInTheQoSOperation:
		return "Syntactical Error In The QoS Operation"
	case nasMessage.Cause5GSMInvalidMappedEPSBearerIdentity:
		return "Invalid Mapped EPS Bearer Identity"
	case nasMessage.Cause5GSMSemanticallyIncorrectMessage:
		return "Semantically Incorrect Message"
	case nasMessage.Cause5GSMInvalidMandatoryInformation:
		return "Invalid Mandatory Information"
	case nasMessage.Cause5GSMMessageTypeNonExistentOrNotImplemented:
		return "Message Type Non Existent Or Not Implemented"
	case nasMessage.Cause5GSMMessageTypeNotCompatibleWithTheProtocolState:
		return "Message Type Not Compatible With The Protocol State"
	case nasMessage.Cause5GSMInformationElementNonExistentOrNotImplemented:
		return "Information Element Non Existent Or Not Implemented"
	case nasMessage.Cause5GSMConditionalIEError:
		return "Conditional IE Error"
	case nasMessage.Cause5GSMMessageNotCompatibleWithTheProtocolState:
		return "Message Not Compatible With The Protocol State"
	case nasMessage.Cause5GSMProtocolErrorUnspecified:
		return "Protocol Error Unspecified"
	default:
		return fmt.Sprintf("Unknown(%d)", causeValue)
	}
}
