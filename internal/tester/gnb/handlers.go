// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	ngaplib "github.com/ellanetworks/core/ngap"
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
	gnb.cond.Broadcast()
}

func HandleFrame(gnb *GnodeB, sctpFrame SCTPFrame) error {
	pdu, err := ngap.Decoder(sctpFrame.Data)
	if err != nil {
		return fmt.Errorf("could not decode NGAP: %v", err)
	}

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		err := handleNGAPInitiatingMessage(gnb, pdu, sctpFrame.Data)
		if err != nil {
			return fmt.Errorf("could not handle NGAP InitiatingMessage: %v", err)
		}

		updateReceivedFramesMap(gnb, pdu.Present, pdu.InitiatingMessage.Value.Present, sctpFrame)

		return nil
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		err := handleNGAPSuccessfulOutcome(pdu, sctpFrame.Data)
		if err != nil {
			return fmt.Errorf("could not handle NGAP SuccessfulOutcome: %v", err)
		}

		updateReceivedFramesMap(gnb, pdu.Present, pdu.SuccessfulOutcome.Value.Present, sctpFrame)

		return nil
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		err := handleNGAPUnsuccessfulOutcome(pdu, sctpFrame.Data)
		if err != nil {
			return fmt.Errorf("could not handle NGAP UnsuccessfulOutcome: %v", err)
		}

		updateReceivedFramesMap(gnb, pdu.Present, pdu.UnsuccessfulOutcome.Value.Present, sctpFrame)

		return nil

	default:
		return fmt.Errorf("NGAP PDU Present is invalid: %d", pdu.Present)
	}
}

func handleNGAPInitiatingMessage(gnb *GnodeB, pdu *ngapType.NGAPPDU, raw []byte) error {
	switch pdu.InitiatingMessage.Value.Present {
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		return handleDownlinkNASTransport(gnb, pdu.InitiatingMessage.Value.DownlinkNASTransport)
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		return handleInitialContextSetupRequest(gnb, outcomeValue(raw))
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		return handlePDUSessionResourceSetupRequest(gnb, pdu.InitiatingMessage.Value.PDUSessionResourceSetupRequest)
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyRequest:
		return handlePDUSessionResourceModifyRequest(gnb, pdu.InitiatingMessage.Value.PDUSessionResourceModifyRequest)
	case ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand:
		return handlePDUSessionResourceReleaseCommand(gnb, pdu.InitiatingMessage.Value.PDUSessionResourceReleaseCommand)
	case ngapType.InitiatingMessagePresentUEContextReleaseCommand:
		return handleUEContextReleaseCommand(gnb, outcomeValue(raw))
	case ngapType.InitiatingMessagePresentPaging:
		return handlePaging(gnb, pdu.InitiatingMessage.Value.Paging)
	case ngapType.InitiatingMessagePresentErrorIndication:
		return handleErrorIndication(outcomeValue(raw))
	case ngapType.InitiatingMessagePresentHandoverRequest:
		return handleHandoverRequest(gnb, pdu.InitiatingMessage.Value.HandoverRequest)
	case ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport:
		return handleDownlinkUEAssociatedNRPPaTransport(gnb, pdu.InitiatingMessage.Value.DownlinkUEAssociatedNRPPaTransport)
	default:
		return fmt.Errorf("NGAP InitiatingMessage Present is invalid: %d", pdu.InitiatingMessage.Value.Present)
	}
}

// raw is the PDU as received. Procedures already migrated to the in-house
// codec parse it themselves; the free5gc decode above is still what keys
// received frames for the scenarios' WaitForMessage.
func handleNGAPSuccessfulOutcome(pdu *ngapType.NGAPPDU, raw []byte) error {
	switch pdu.SuccessfulOutcome.Value.Present {
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		return handleNGSetupResponse(outcomeValue(raw))
	case ngapType.SuccessfulOutcomePresentNGResetAcknowledge:
		return handleNGResetAcknowledge(outcomeValue(raw))
	case ngapType.SuccessfulOutcomePresentRANConfigurationUpdateAcknowledge:
		return nil // Handled via WaitForMessage
	case ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge:
		return nil // Handled via WaitForMessage
	case ngapType.SuccessfulOutcomePresentHandoverCommand:
		return nil // Handled via WaitForMessage by source gNB
	default:
		return fmt.Errorf("NGAP SuccessfulOutcome Present is invalid: %d", pdu.SuccessfulOutcome.Value.Present)
	}
}

func handleNGAPUnsuccessfulOutcome(pdu *ngapType.NGAPPDU, raw []byte) error {
	switch pdu.UnsuccessfulOutcome.Value.Present {
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		return handleNGSetupFailure(outcomeValue(raw))
	case ngapType.UnsuccessfulOutcomePresentRANConfigurationUpdateFailure:
		return nil // Handled via WaitForMessage
	case ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure:
		return nil // Handled via WaitForMessage
	default:
		return fmt.Errorf("NGAP UnsuccessfulOutcome Present is invalid: %d", pdu.UnsuccessfulOutcome.Value.Present)
	}
}

// outcomeValue returns the open-type payload of an NGAP PDU, or nil when the
// envelope does not decode — the caller's parse then reports it.
func outcomeValue(raw []byte) []byte {
	pdu, err := ngaplib.Unmarshal(raw)
	if err != nil {
		return nil
	}

	switch m := pdu.(type) {
	case *ngaplib.SuccessfulOutcome:
		return m.Value
	case *ngaplib.UnsuccessfulOutcome:
		return m.Value
	case *ngaplib.InitiatingMessage:
		return m.Value
	}

	return nil
}
