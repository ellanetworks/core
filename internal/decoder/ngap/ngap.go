package ngap

import (
	"fmt"

	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
)

type EnumField struct {
	Type    string `json:"type"` // always "enum"
	Value   int    `json:"value"`
	Label   string `json:"label"`
	Unknown bool   `json:"unknown"`
}

func makeEnum(v int, label string, unknown bool) EnumField {
	return EnumField{Type: "enum", Value: v, Label: label, Unknown: unknown}
}

type NGAPMessageValue struct {
	IEs   []IE   `json:"ies,omitempty"`
	Error string `json:"error,omitempty"`
}

type NGAPMessage struct {
	PDUType       string           `json:"pdu_type"`
	ProcedureCode EnumField        `json:"procedure_code"`
	Criticality   EnumField        `json:"criticality"`
	Value         NGAPMessageValue `json:"value"`
}

func DecodeNGAPMessage(raw []byte) NGAPMessage {
	pdu, err := ngap.Decoder(raw)
	if err != nil {
		return NGAPMessage{
			Value: NGAPMessageValue{
				Error: fmt.Sprintf("Could not decode raw ngap message: %v", err),
			},
		}
	}

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		return NGAPMessage{
			ProcedureCode: procedureCodeToEnum(pdu.InitiatingMessage.ProcedureCode.Value),
			Criticality:   criticalityToEnum(pdu.InitiatingMessage.Criticality.Value),
			PDUType:       "InitiatingMessage",
			Value:         buildInitiatingMessage(*pdu.InitiatingMessage),
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		return NGAPMessage{
			ProcedureCode: procedureCodeToEnum(pdu.SuccessfulOutcome.ProcedureCode.Value),
			Criticality:   criticalityToEnum(pdu.SuccessfulOutcome.Criticality.Value),
			PDUType:       "SuccessfulOutcome",
			Value:         buildSuccessfulOutcome(*pdu.SuccessfulOutcome),
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		return NGAPMessage{
			ProcedureCode: procedureCodeToEnum(pdu.UnsuccessfulOutcome.ProcedureCode.Value),
			Criticality:   criticalityToEnum(pdu.UnsuccessfulOutcome.Criticality.Value),
			PDUType:       "UnsuccessfulOutcome",
			Value:         buildUnsuccessfulOutcome(*pdu.UnsuccessfulOutcome),
		}
	default:
		return NGAPMessage{
			PDUType: "Unknown",
			Value: NGAPMessageValue{
				Error: fmt.Sprintf("unknown NGAP PDU type: %d", pdu.Present),
			},
		}
	}
}

func buildInitiatingMessage(initMsg ngapType.InitiatingMessage) NGAPMessageValue {
	switch initMsg.Value.Present {
	case ngapType.InitiatingMessagePresentNGSetupRequest:
		return buildNGSetupRequest(*initMsg.Value.NGSetupRequest)
	case ngapType.InitiatingMessagePresentInitialUEMessage:
		return buildInitialUEMessage(*initMsg.Value.InitialUEMessage)
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		return buildDownlinkNASTransport(*initMsg.Value.DownlinkNASTransport)
	case ngapType.InitiatingMessagePresentUplinkNASTransport:
		return buildUplinkNASTransport(*initMsg.Value.UplinkNASTransport)
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		return buildInitialContextSetupRequest(*initMsg.Value.InitialContextSetupRequest)
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		return buildPDUSessionResourceSetupRequest(*initMsg.Value.PDUSessionResourceSetupRequest)
	default:
		return NGAPMessageValue{
			Error: fmt.Sprintf("Unsupported message %d", initMsg.Value.Present),
		}
	}
}

func buildSuccessfulOutcome(sucMsg ngapType.SuccessfulOutcome) NGAPMessageValue {
	switch sucMsg.Value.Present {
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		return buildNGSetupResponse(*sucMsg.Value.NGSetupResponse)
	case ngapType.SuccessfulOutcomePresentInitialContextSetupResponse:
		return buildInitialContextSetupResponse(*sucMsg.Value.InitialContextSetupResponse)
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceSetupResponse:
		return buildPDUSessionResourceSetupResponse(*sucMsg.Value.PDUSessionResourceSetupResponse)
	default:
		return NGAPMessageValue{
			Error: fmt.Sprintf("Unsupported message %d", sucMsg.Value.Present),
		}
	}
}

func buildUnsuccessfulOutcome(unsucMsg ngapType.UnsuccessfulOutcome) NGAPMessageValue {
	switch unsucMsg.Value.Present {
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		return buildNGSetupFailure(*unsucMsg.Value.NGSetupFailure)
	default:
		return NGAPMessageValue{
			Error: fmt.Sprintf("Unsupported message %d", unsucMsg.Value.Present),
		}
	}
}

func criticalityToEnum(c aper.Enumerated) EnumField {
	switch c {
	case ngapType.CriticalityPresentReject:
		return makeEnum(int(c), "Reject", false)
	case ngapType.CriticalityPresentIgnore:
		return makeEnum(int(c), "Ignore", false)
	case ngapType.CriticalityPresentNotify:
		return makeEnum(int(c), "Notify", false)
	default:
		return makeEnum(int(c), "", true)
	}
}

func procedureCodeToEnum(code int64) EnumField {
	switch code {
	case ngapType.ProcedureCodeAMFConfigurationUpdate:
		return makeEnum(int(code), "AMFConfigurationUpdate", false)
	case ngapType.ProcedureCodeAMFStatusIndication:
		return makeEnum(int(code), "AMFStatusIndication", false)
	case ngapType.ProcedureCodeCellTrafficTrace:
		return makeEnum(int(code), "CellTrafficTrace", false)
	case ngapType.ProcedureCodeDeactivateTrace:
		return makeEnum(int(code), "DeactivateTrace", false)
	case ngapType.ProcedureCodeDownlinkNASTransport:
		return makeEnum(int(code), "DownlinkNASTransport", false)
	case ngapType.ProcedureCodeDownlinkNonUEAssociatedNRPPaTransport:
		return makeEnum(int(code), "DownlinkNonUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeDownlinkRANConfigurationTransfer:
		return makeEnum(int(code), "DownlinkRANConfigurationTransfer", false)
	case ngapType.ProcedureCodeDownlinkRANStatusTransfer:
		return makeEnum(int(code), "DownlinkRANStatusTransfer", false)
	case ngapType.ProcedureCodeDownlinkUEAssociatedNRPPaTransport:
		return makeEnum(int(code), "DownlinkUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeErrorIndication:
		return makeEnum(int(code), "ErrorIndication", false)
	case ngapType.ProcedureCodeHandoverCancel:
		return makeEnum(int(code), "HandoverCancel", false)
	case ngapType.ProcedureCodeHandoverNotification:
		return makeEnum(int(code), "HandoverNotification", false)
	case ngapType.ProcedureCodeHandoverPreparation:
		return makeEnum(int(code), "HandoverPreparation", false)
	case ngapType.ProcedureCodeHandoverResourceAllocation:
		return makeEnum(int(code), "HandoverResourceAllocation", false)
	case ngapType.ProcedureCodeInitialContextSetup:
		return makeEnum(int(code), "InitialContextSetup", false)
	case ngapType.ProcedureCodeInitialUEMessage:
		return makeEnum(int(code), "InitialUEMessage", false)
	case ngapType.ProcedureCodeLocationReportingControl:
		return makeEnum(int(code), "LocationReportingControl", false)
	case ngapType.ProcedureCodeLocationReportingFailureIndication:
		return makeEnum(int(code), "LocationReportingFailureIndication", false)
	case ngapType.ProcedureCodeLocationReport:
		return makeEnum(int(code), "LocationReport", false)
	case ngapType.ProcedureCodeNASNonDeliveryIndication:
		return makeEnum(int(code), "NASNonDeliveryIndication", false)
	case ngapType.ProcedureCodeNGReset:
		return makeEnum(int(code), "NGReset", false)
	case ngapType.ProcedureCodeNGSetup:
		return makeEnum(int(code), "NGSetup", false)
	case ngapType.ProcedureCodeOverloadStart:
		return makeEnum(int(code), "OverloadStart", false)
	case ngapType.ProcedureCodeOverloadStop:
		return makeEnum(int(code), "OverloadStop", false)
	case ngapType.ProcedureCodePaging:
		return makeEnum(int(code), "Paging", false)
	case ngapType.ProcedureCodePathSwitchRequest:
		return makeEnum(int(code), "PathSwitchRequest", false)
	case ngapType.ProcedureCodePDUSessionResourceModify:
		return makeEnum(int(code), "PDUSessionResourceModify", false)
	case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
		return makeEnum(int(code), "PDUSessionResourceModifyIndication", false)
	case ngapType.ProcedureCodePDUSessionResourceRelease:
		return makeEnum(int(code), "PDUSessionResourceRelease", false)
	case ngapType.ProcedureCodePDUSessionResourceSetup:
		return makeEnum(int(code), "PDUSessionResourceSetup", false)
	case ngapType.ProcedureCodePDUSessionResourceNotify:
		return makeEnum(int(code), "PDUSessionResourceNotify", false)
	case ngapType.ProcedureCodePrivateMessage:
		return makeEnum(int(code), "PrivateMessage", false)
	case ngapType.ProcedureCodePWSCancel:
		return makeEnum(int(code), "PWSCancel", false)
	case ngapType.ProcedureCodePWSFailureIndication:
		return makeEnum(int(code), "PWSFailureIndication", false)
	case ngapType.ProcedureCodePWSRestartIndication:
		return makeEnum(int(code), "PWSRestartIndication", false)
	case ngapType.ProcedureCodeRANConfigurationUpdate:
		return makeEnum(int(code), "RANConfigurationUpdate", false)
	case ngapType.ProcedureCodeRerouteNASRequest:
		return makeEnum(int(code), "RerouteNASRequest", false)
	case ngapType.ProcedureCodeRRCInactiveTransitionReport:
		return makeEnum(int(code), "RRCInactiveTransitionReport", false)
	case ngapType.ProcedureCodeTraceFailureIndication:
		return makeEnum(int(code), "TraceFailureIndication", false)
	case ngapType.ProcedureCodeTraceStart:
		return makeEnum(int(code), "TraceStart", false)
	case ngapType.ProcedureCodeUEContextModification:
		return makeEnum(int(code), "UEContextModification", false)
	case ngapType.ProcedureCodeUEContextRelease:
		return makeEnum(int(code), "UEContextRelease", false)
	case ngapType.ProcedureCodeUEContextReleaseRequest:
		return makeEnum(int(code), "UEContextReleaseRequest", false)
	case ngapType.ProcedureCodeUERadioCapabilityCheck:
		return makeEnum(int(code), "UERadioCapabilityCheck", false)
	case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
		return makeEnum(int(code), "UERadioCapabilityInfoIndication", false)
	case ngapType.ProcedureCodeUETNLABindingRelease:
		return makeEnum(int(code), "UETNLABindingRelease", false)
	case ngapType.ProcedureCodeUplinkNASTransport:
		return makeEnum(int(code), "UplinkNASTransport", false)
	case ngapType.ProcedureCodeUplinkNonUEAssociatedNRPPaTransport:
		return makeEnum(int(code), "UplinkNonUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
		return makeEnum(int(code), "UplinkRANConfigurationTransfer", false)
	case ngapType.ProcedureCodeUplinkRANStatusTransfer:
		return makeEnum(int(code), "UplinkRANStatusTransfer", false)
	case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
		return makeEnum(int(code), "UplinkUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeWriteReplaceWarning:
		return makeEnum(int(code), "WriteReplaceWarning", false)
	case ngapType.ProcedureCodeSecondaryRATDataUsageReport:
		return makeEnum(int(code), "SecondaryRATDataUsageReport", false)
	default:
		return makeEnum(int(code), "", true)
	}
}
