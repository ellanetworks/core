package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
)

type NGAPMessageValue struct {
	IEs   []IE   `json:"ies,omitempty"`
	Error string `json:"error,omitempty"` // reserved field for decoding errors
}

type NGAPMessage struct {
	PDUType       string                  `json:"pdu_type"`
	ProcedureCode utils.EnumField[int64]  `json:"procedure_code"`
	Criticality   utils.EnumField[uint64] `json:"criticality"`
	Value         NGAPMessageValue        `json:"value"`
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

func criticalityToEnum(c aper.Enumerated) utils.EnumField[uint64] {
	switch c {
	case ngapType.CriticalityPresentReject:
		return utils.MakeEnum(uint64(c), "Reject", false)
	case ngapType.CriticalityPresentIgnore:
		return utils.MakeEnum(uint64(c), "Ignore", false)
	case ngapType.CriticalityPresentNotify:
		return utils.MakeEnum(uint64(c), "Notify", false)
	default:
		return utils.MakeEnum(uint64(c), "", true)
	}
}

func procedureCodeToEnum(code int64) utils.EnumField[int64] {
	switch code {
	case ngapType.ProcedureCodeAMFConfigurationUpdate:
		return utils.MakeEnum(code, "AMFConfigurationUpdate", false)
	case ngapType.ProcedureCodeAMFStatusIndication:
		return utils.MakeEnum(code, "AMFStatusIndication", false)
	case ngapType.ProcedureCodeCellTrafficTrace:
		return utils.MakeEnum(code, "CellTrafficTrace", false)
	case ngapType.ProcedureCodeDeactivateTrace:
		return utils.MakeEnum(code, "DeactivateTrace", false)
	case ngapType.ProcedureCodeDownlinkNASTransport:
		return utils.MakeEnum(code, "DownlinkNASTransport", false)
	case ngapType.ProcedureCodeDownlinkNonUEAssociatedNRPPaTransport:
		return utils.MakeEnum(code, "DownlinkNonUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeDownlinkRANConfigurationTransfer:
		return utils.MakeEnum(code, "DownlinkRANConfigurationTransfer", false)
	case ngapType.ProcedureCodeDownlinkRANStatusTransfer:
		return utils.MakeEnum(code, "DownlinkRANStatusTransfer", false)
	case ngapType.ProcedureCodeDownlinkUEAssociatedNRPPaTransport:
		return utils.MakeEnum(code, "DownlinkUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeErrorIndication:
		return utils.MakeEnum(code, "ErrorIndication", false)
	case ngapType.ProcedureCodeHandoverCancel:
		return utils.MakeEnum(code, "HandoverCancel", false)
	case ngapType.ProcedureCodeHandoverNotification:
		return utils.MakeEnum(code, "HandoverNotification", false)
	case ngapType.ProcedureCodeHandoverPreparation:
		return utils.MakeEnum(code, "HandoverPreparation", false)
	case ngapType.ProcedureCodeHandoverResourceAllocation:
		return utils.MakeEnum(code, "HandoverResourceAllocation", false)
	case ngapType.ProcedureCodeInitialContextSetup:
		return utils.MakeEnum(code, "InitialContextSetup", false)
	case ngapType.ProcedureCodeInitialUEMessage:
		return utils.MakeEnum(code, "InitialUEMessage", false)
	case ngapType.ProcedureCodeLocationReportingControl:
		return utils.MakeEnum(code, "LocationReportingControl", false)
	case ngapType.ProcedureCodeLocationReportingFailureIndication:
		return utils.MakeEnum(code, "LocationReportingFailureIndication", false)
	case ngapType.ProcedureCodeLocationReport:
		return utils.MakeEnum(code, "LocationReport", false)
	case ngapType.ProcedureCodeNASNonDeliveryIndication:
		return utils.MakeEnum(code, "NASNonDeliveryIndication", false)
	case ngapType.ProcedureCodeNGReset:
		return utils.MakeEnum(code, "NGReset", false)
	case ngapType.ProcedureCodeNGSetup:
		return utils.MakeEnum(code, "NGSetup", false)
	case ngapType.ProcedureCodeOverloadStart:
		return utils.MakeEnum(code, "OverloadStart", false)
	case ngapType.ProcedureCodeOverloadStop:
		return utils.MakeEnum(code, "OverloadStop", false)
	case ngapType.ProcedureCodePaging:
		return utils.MakeEnum(code, "Paging", false)
	case ngapType.ProcedureCodePathSwitchRequest:
		return utils.MakeEnum(code, "PathSwitchRequest", false)
	case ngapType.ProcedureCodePDUSessionResourceModify:
		return utils.MakeEnum(code, "PDUSessionResourceModify", false)
	case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
		return utils.MakeEnum(code, "PDUSessionResourceModifyIndication", false)
	case ngapType.ProcedureCodePDUSessionResourceRelease:
		return utils.MakeEnum(code, "PDUSessionResourceRelease", false)
	case ngapType.ProcedureCodePDUSessionResourceSetup:
		return utils.MakeEnum(code, "PDUSessionResourceSetup", false)
	case ngapType.ProcedureCodePDUSessionResourceNotify:
		return utils.MakeEnum(code, "PDUSessionResourceNotify", false)
	case ngapType.ProcedureCodePrivateMessage:
		return utils.MakeEnum(code, "PrivateMessage", false)
	case ngapType.ProcedureCodePWSCancel:
		return utils.MakeEnum(code, "PWSCancel", false)
	case ngapType.ProcedureCodePWSFailureIndication:
		return utils.MakeEnum(code, "PWSFailureIndication", false)
	case ngapType.ProcedureCodePWSRestartIndication:
		return utils.MakeEnum(code, "PWSRestartIndication", false)
	case ngapType.ProcedureCodeRANConfigurationUpdate:
		return utils.MakeEnum(code, "RANConfigurationUpdate", false)
	case ngapType.ProcedureCodeRerouteNASRequest:
		return utils.MakeEnum(code, "RerouteNASRequest", false)
	case ngapType.ProcedureCodeRRCInactiveTransitionReport:
		return utils.MakeEnum(code, "RRCInactiveTransitionReport", false)
	case ngapType.ProcedureCodeTraceFailureIndication:
		return utils.MakeEnum(code, "TraceFailureIndication", false)
	case ngapType.ProcedureCodeTraceStart:
		return utils.MakeEnum(code, "TraceStart", false)
	case ngapType.ProcedureCodeUEContextModification:
		return utils.MakeEnum(code, "UEContextModification", false)
	case ngapType.ProcedureCodeUEContextRelease:
		return utils.MakeEnum(code, "UEContextRelease", false)
	case ngapType.ProcedureCodeUEContextReleaseRequest:
		return utils.MakeEnum(code, "UEContextReleaseRequest", false)
	case ngapType.ProcedureCodeUERadioCapabilityCheck:
		return utils.MakeEnum(code, "UERadioCapabilityCheck", false)
	case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
		return utils.MakeEnum(code, "UERadioCapabilityInfoIndication", false)
	case ngapType.ProcedureCodeUETNLABindingRelease:
		return utils.MakeEnum(code, "UETNLABindingRelease", false)
	case ngapType.ProcedureCodeUplinkNASTransport:
		return utils.MakeEnum(code, "UplinkNASTransport", false)
	case ngapType.ProcedureCodeUplinkNonUEAssociatedNRPPaTransport:
		return utils.MakeEnum(code, "UplinkNonUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
		return utils.MakeEnum(code, "UplinkRANConfigurationTransfer", false)
	case ngapType.ProcedureCodeUplinkRANStatusTransfer:
		return utils.MakeEnum(code, "UplinkRANStatusTransfer", false)
	case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
		return utils.MakeEnum(code, "UplinkUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeWriteReplaceWarning:
		return utils.MakeEnum(code, "WriteReplaceWarning", false)
	case ngapType.ProcedureCodeSecondaryRATDataUsageReport:
		return utils.MakeEnum(code, "SecondaryRATDataUsageReport", false)
	default:
		return utils.MakeEnum(code, "", true)
	}
}
