package ngap

import (
	"fmt"

	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
)

type EnumField struct {
	Type  string `json:"type" default:"enum"`
	Value int    `json:"value"`
	Label string `json:"label"`
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
		return EnumField{Label: "Reject", Value: int(c)}
	case ngapType.CriticalityPresentIgnore:
		return EnumField{Label: "Ignore", Value: int(c)}
	case ngapType.CriticalityPresentNotify:
		return EnumField{Label: "Notify", Value: int(c)}
	default:
		return EnumField{Label: "Unknown", Value: int(c)}
	}
}

func procedureCodeToEnum(code int64) EnumField {
	return EnumField{
		Label: ngapType.ProcedureName(code),
		Value: int(code),
	}
}
