package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/ngap"
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
	case ngapType.InitiatingMessagePresentUEContextReleaseRequest:
		return buildUEContextReleaseRequest(*initMsg.Value.UEContextReleaseRequest)
	case ngapType.InitiatingMessagePresentUEContextReleaseCommand:
		return buildUEContextReleaseCommand(*initMsg.Value.UEContextReleaseCommand)
	case ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand:
		return buildPDUSessionResourceReleaseCommand(*initMsg.Value.PDUSessionResourceReleaseCommand)
	case ngapType.InitiatingMessagePresentUERadioCapabilityInfoIndication:
		return buildUERadioCapabilityInfoIndication(*initMsg.Value.UERadioCapabilityInfoIndication)
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
	case ngapType.SuccessfulOutcomePresentUEContextReleaseComplete:
		return buildUEContextReleaseComplete(*sucMsg.Value.UEContextReleaseComplete)
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceReleaseResponse:
		return buildPDUSessionResourceReleaseResponse(*sucMsg.Value.PDUSessionResourceReleaseResponse)
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
