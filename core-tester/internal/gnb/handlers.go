package gnb

import (
	"fmt"

	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func updateReceivedFramesMap(gnb *GnodeB, pduType int, msgType int, frame SCTPFrame) {
	gnb.mu.Lock()
	defer gnb.mu.Unlock()

	if gnb.receivedFrames == nil {
		gnb.receivedFrames = make(map[int]map[int][]SCTPFrame)
	}

	if gnb.receivedFrames[pduType] == nil {
		gnb.receivedFrames[pduType] = make(map[int][]SCTPFrame)
	}

	gnb.receivedFrames[pduType][msgType] = append(gnb.receivedFrames[pduType][msgType], frame)
}

func HandleFrame(gnb *GnodeB, sctpFrame SCTPFrame) error {
	pdu, err := ngap.Decoder(sctpFrame.Data)
	if err != nil {
		return fmt.Errorf("could not decode NGAP: %v", err)
	}

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		err := handleNGAPInitiatingMessage(gnb, pdu)
		if err != nil {
			return fmt.Errorf("could not handle NGAP InitiatingMessage: %v", err)
		}

		updateReceivedFramesMap(gnb, pdu.Present, pdu.InitiatingMessage.Value.Present, sctpFrame)

		return nil
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		err := handleNGAPSuccessfulOutcome(pdu)
		if err != nil {
			return fmt.Errorf("could not handle NGAP SuccessfulOutcome: %v", err)
		}

		updateReceivedFramesMap(gnb, pdu.Present, pdu.SuccessfulOutcome.Value.Present, sctpFrame)

		return nil
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		err := handleNGAPUnsuccessfulOutcome(pdu)
		if err != nil {
			return fmt.Errorf("could not handle NGAP UnsuccessfulOutcome: %v", err)
		}

		updateReceivedFramesMap(gnb, pdu.Present, pdu.UnsuccessfulOutcome.Value.Present, sctpFrame)

		return nil

	default:
		return fmt.Errorf("NGAP PDU Present is invalid: %d", pdu.Present)
	}
}

func handleNGAPInitiatingMessage(gnb *GnodeB, pdu *ngapType.NGAPPDU) error {
	switch pdu.InitiatingMessage.Value.Present {
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		return handleDownlinkNASTransport(gnb, pdu.InitiatingMessage.Value.DownlinkNASTransport)
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		return handleInitialContextSetupRequest(gnb, pdu.InitiatingMessage.Value.InitialContextSetupRequest)
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		return handlePDUSessionResourceSetupRequest(gnb, pdu.InitiatingMessage.Value.PDUSessionResourceSetupRequest)
	case ngapType.InitiatingMessagePresentUEContextReleaseCommand:
		return handleUEContextReleaseCommand(gnb, pdu.InitiatingMessage.Value.UEContextReleaseCommand)
	case ngapType.InitiatingMessagePresentPaging:
		return handlePaging(gnb, pdu.InitiatingMessage.Value.Paging)
	case ngapType.InitiatingMessagePresentErrorIndication:
		return handleErrorIndication(pdu.InitiatingMessage.Value.ErrorIndication)
	default:
		return fmt.Errorf("NGAP InitiatingMessage Present is invalid: %d", pdu.InitiatingMessage.Value.Present)
	}
}

func handleNGAPSuccessfulOutcome(pdu *ngapType.NGAPPDU) error {
	switch pdu.SuccessfulOutcome.Value.Present {
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		return handleNGSetupResponse(pdu.SuccessfulOutcome.Value.NGSetupResponse)
	case ngapType.SuccessfulOutcomePresentNGResetAcknowledge:
		return handleNGResetAcknowledge(pdu.SuccessfulOutcome.Value.NGResetAcknowledge)
	case ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge:
		return nil // Handled via WaitForMessage
	default:
		return fmt.Errorf("NGAP SuccessfulOutcome Present is invalid: %d", pdu.SuccessfulOutcome.Value.Present)
	}
}

func handleNGAPUnsuccessfulOutcome(pdu *ngapType.NGAPPDU) error {
	switch pdu.UnsuccessfulOutcome.Value.Present {
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		return handleNGSetupFailure(pdu.UnsuccessfulOutcome.Value.NGSetupFailure)
	case ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure:
		return nil // Handled via WaitForMessage
	default:
		return fmt.Errorf("NGAP UnsuccessfulOutcome Present is invalid: %d", pdu.UnsuccessfulOutcome.Value.Present)
	}
}
