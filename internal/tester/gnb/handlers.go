// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"github.com/ellanetworks/core/ngap"
)

func updateReceivedFramesMap(gnb *GnodeB, frame SCTPFrame) {
	gnb.mu.Lock()
	defer gnb.mu.Unlock()

	if gnb.receivedFrames == nil {
		gnb.receivedFrames = make(map[Category]map[ngap.ProcedureCode][]SCTPFrame)
	}

	if gnb.receivedFrames[frame.Category] == nil {
		gnb.receivedFrames[frame.Category] = make(map[ngap.ProcedureCode][]SCTPFrame)
	}

	byCode := gnb.receivedFrames[frame.Category]
	byCode[frame.ProcedureCode] = append(byCode[frame.ProcedureCode], frame)

	gnb.cond.Broadcast()
}

// HandleFrame runs the handler for the procedures this simulator answers on
// its own, and files the frame for WaitForMessage. The frame must already be
// decoded (see decodeFrame).
func HandleFrame(gnb *GnodeB, sctpFrame SCTPFrame) error {
	defer updateReceivedFramesMap(gnb, sctpFrame)

	return handleFrame(gnb, sctpFrame)
}

// handleFrame answers the procedures this simulator acts on by itself.
// Everything else is left to WaitForMessage, which the scenarios drive.
func handleFrame(gnb *GnodeB, frame SCTPFrame) error {
	switch frame.Category {
	case Initiating:
		return handleInitiatingMessage(gnb, frame)
	case Successful:
		switch frame.ProcedureCode {
		case ngap.ProcNGSetup:
			return handleNGSetupResponse(frame.Value)
		case ngap.ProcNGReset:
			return handleNGResetAcknowledge(frame.Value)
		}
	case Unsuccessful:
		if frame.ProcedureCode == ngap.ProcNGSetup {
			return handleNGSetupFailure(frame.Value)
		}
	}

	return nil
}

func handleInitiatingMessage(gnb *GnodeB, frame SCTPFrame) error {
	switch frame.ProcedureCode {
	case ngap.ProcDownlinkNASTransport:
		return handleDownlinkNASTransport(gnb, frame.Value)
	case ngap.ProcInitialContextSetup:
		return handleInitialContextSetupRequest(gnb, frame.Value)
	case ngap.ProcPDUSessionResourceSetup:
		return handlePDUSessionResourceSetupRequest(gnb, frame.Value)
	case ngap.ProcPDUSessionResourceModify:
		return handlePDUSessionResourceModifyRequest(gnb, frame.Value)
	case ngap.ProcPDUSessionResourceRelease:
		return handlePDUSessionResourceReleaseCommand(gnb, frame.Value)
	case ngap.ProcUEContextRelease:
		return handleUEContextReleaseCommand(gnb, frame.Value)
	case ngap.ProcPaging:
		return handlePaging(gnb, frame.Value)
	case ngap.ProcErrorIndication:
		return handleErrorIndication(frame.Value)
	case ngap.ProcHandoverResourceAllocation:
		return handleHandoverRequest(gnb, frame.Value)
	case ngap.ProcDownlinkUEAssociatedNRPPaTransport:
		return handleDownlinkUEAssociatedNRPPaTransport(gnb, frame.Value)
	}

	return nil
}
