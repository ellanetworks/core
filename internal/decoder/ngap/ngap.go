package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

type NGAPMessageValue struct {
	IEs   []IE   `json:"ies,omitempty"`
	Error string `json:"error,omitempty"` // reserved field for decoding errors
}

type NGAPMessage struct {
	Summary       string                  `json:"summary,omitempty"`
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

	var msg NGAPMessage

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		value := buildInitiatingMessage(*pdu.InitiatingMessage)
		setIEValueTypes(value.IEs)
		msg = NGAPMessage{
			PDUType:       "InitiatingMessage",
			ProcedureCode: procedureCodeToEnum(pdu.InitiatingMessage.ProcedureCode.Value),
			Criticality:   criticalityToEnum(pdu.InitiatingMessage.Criticality.Value),
			Value:         value,
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		value := buildSuccessfulOutcome(*pdu.SuccessfulOutcome)
		setIEValueTypes(value.IEs)
		msg = NGAPMessage{
			PDUType:       "SuccessfulOutcome",
			ProcedureCode: procedureCodeToEnum(pdu.SuccessfulOutcome.ProcedureCode.Value),
			Criticality:   criticalityToEnum(pdu.SuccessfulOutcome.Criticality.Value),
			Value:         value,
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		value := buildUnsuccessfulOutcome(*pdu.UnsuccessfulOutcome)
		setIEValueTypes(value.IEs)
		msg = NGAPMessage{
			PDUType:       "UnsuccessfulOutcome",
			ProcedureCode: procedureCodeToEnum(pdu.UnsuccessfulOutcome.ProcedureCode.Value),
			Criticality:   criticalityToEnum(pdu.UnsuccessfulOutcome.Criticality.Value),
			Value:         value,
		}
	default:
		return NGAPMessage{
			PDUType: "Unknown",
			Value: NGAPMessageValue{
				Error: fmt.Sprintf("unknown NGAP PDU type: %d", pdu.Present),
			},
		}
	}

	msg.Summary = buildNGAPSummary(msg)

	return msg
}

// buildNGAPSummary generates a one-line summary from the
// procedure code and key IEs. Example: "InitialUEMessage, RAN-UE=1, NAS=RegistrationRequest"
func buildNGAPSummary(msg NGAPMessage) string {
	summary := msg.ProcedureCode.Label
	if summary == "" {
		summary = msg.PDUType
	}

	for _, ie := range msg.Value.IEs {
		switch ie.ID.Label {
		case "AMFUENGAPID":
			summary += fmt.Sprintf(", AMF-UE=%d", ie.Value)
		case "RANUENGAPID":
			summary += fmt.Sprintf(", RAN-UE=%d", ie.Value)
		case "NASPDU":
			if nasPdu, ok := ie.Value.(NASPDU); ok && nasPdu.Decoded != nil {
				summary += ", NAS=" + nasMessageTypeName(nasPdu.Decoded)
			}
		case "Cause":
			if causeEnum, ok := ie.Value.(utils.EnumField[uint64]); ok {
				summary += ", Cause=" + causeEnum.Label
			}
		}
	}

	return summary
}

func nasMessageTypeName(msg *nas.NASMessage) string {
	if msg.Encrypted {
		return "Encrypted"
	}

	if msg.GmmMessage != nil {
		return msg.GmmMessage.GmmHeader.MessageType.Label
	}

	if msg.GsmMessage != nil {
		return msg.GsmMessage.GsmHeader.MessageType.Label
	}

	return "Unknown"
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
	case ngapType.InitiatingMessagePresentAMFStatusIndication:
		return buildAMFStatusIndication(*initMsg.Value.AMFStatusIndication)
	case ngapType.InitiatingMessagePresentPaging:
		return buildPaging(*initMsg.Value.Paging)
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
	case ngapType.UnsuccessfulOutcomePresentInitialContextSetupFailure:
		return buildInitialContextSetupFailure(*unsucMsg.Value.InitialContextSetupFailure)
	default:
		return NGAPMessageValue{
			Error: fmt.Sprintf("Unsupported message %d", unsucMsg.Value.Present),
		}
	}
}
