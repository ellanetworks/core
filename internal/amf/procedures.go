// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "fmt"

// NGAPPPID is the SCTP payload protocol identifier for NGAP (TS 38.412).
const NGAPPPID uint32 = 60

type NGAPProcedure string

const (
	// Non-UE associated NGAP procedures
	NGAPProcedureNGSetupResponse                   NGAPProcedure = "NGSetupResponse"
	NGAPProcedureNGSetupFailure                    NGAPProcedure = "NGSetupFailure"
	NGAPProcedurePaging                            NGAPProcedure = "Paging"
	NGAPProcedureNGResetAcknowledge                NGAPProcedure = "NGResetAcknowledge"
	NGAPProcedureErrorIndication                   NGAPProcedure = "ErrorIndication"
	NGAPProcedureRANConfigurationUpdateAcknowledge NGAPProcedure = "RANConfigurationUpdateAcknowledge"
	NGAPProcedureRANConfigurationUpdateFailure     NGAPProcedure = "RANConfigurationUpdateFailure"
	NGAPProcedureAMFStatusIndication               NGAPProcedure = "AMFStatusIndication"
	NGAPProcedureDownlinkRANConfigurationTransfer  NGAPProcedure = "DownlinkRANConfigurationTransfer"

	// UE-associated NGAP procedures
	NGAPProcedureInitialContextSetupRequest       NGAPProcedure = "InitialContextSetupRequest"
	NGAPProcedurePDUSessionResourceModifyRequest  NGAPProcedure = "PDUSessionResourceModifyRequest"
	NGAPProcedurePDUSessionResourceModifyConfirm  NGAPProcedure = "PDUSessionResourceModifyConfirm"
	NGAPProcedurePDUSessionResourceSetupRequest   NGAPProcedure = "PDUSessionResourceSetupRequest"
	NGAPProcedurePDUSessionResourceReleaseCommand NGAPProcedure = "PDUSessionResourceReleaseCommand"
	NGAPProcedureDownlinkNASTransport             NGAPProcedure = "DownlinkNASTransport"
	NGAPProcedureLocationReportingControl         NGAPProcedure = "LocationReportingControl"
	NGAPProcedurePathSwitchRequestFailure         NGAPProcedure = "PathSwitchRequestFailure"
	NGAPProcedurePathSwitchRequestAcknowledge     NGAPProcedure = "PathSwitchRequestAcknowledge"
	NGAPProcedureHandoverRequest                  NGAPProcedure = "HandoverRequest"
	NGAPProcedureHandoverCommand                  NGAPProcedure = "HandoverCommand"
	NGAPProcedureHandoverCancelAcknowledge        NGAPProcedure = "HandoverCancelAcknowledge"
	NGAPProcedureHandoverPreparationFailure       NGAPProcedure = "HandoverPreparationFailure"
	NGAPProcedureUEContextReleaseCommand          NGAPProcedure = "UEContextReleaseCommand"
	NGAPProcedureDownlinkNRPPaTransport           NGAPProcedure = "DownlinkNRPPaTransport"
	NGAPProcedureDownlinkRANStatusTransfer        NGAPProcedure = "DownlinkRANStatusTransfer"
)

func GetSCTPStreamID(msgType NGAPProcedure) (uint16, error) {
	switch msgType {
	case NGAPProcedureNGSetupResponse, NGAPProcedureNGSetupFailure,
		NGAPProcedurePaging, NGAPProcedureNGResetAcknowledge,
		NGAPProcedureErrorIndication, NGAPProcedureRANConfigurationUpdateAcknowledge,
		NGAPProcedureRANConfigurationUpdateFailure, NGAPProcedureAMFStatusIndication,
		NGAPProcedureDownlinkRANConfigurationTransfer:
		return 0, nil

	case NGAPProcedureInitialContextSetupRequest, NGAPProcedureUEContextReleaseCommand,
		NGAPProcedureDownlinkNASTransport, NGAPProcedurePDUSessionResourceSetupRequest,
		NGAPProcedurePDUSessionResourceReleaseCommand, NGAPProcedureHandoverRequest,
		NGAPProcedureHandoverCommand, NGAPProcedureHandoverPreparationFailure,
		NGAPProcedurePathSwitchRequestAcknowledge, NGAPProcedurePDUSessionResourceModifyRequest,
		NGAPProcedurePDUSessionResourceModifyConfirm, NGAPProcedureHandoverCancelAcknowledge,
		NGAPProcedureLocationReportingControl, NGAPProcedurePathSwitchRequestFailure,
		NGAPProcedureDownlinkNRPPaTransport,
		NGAPProcedureDownlinkRANStatusTransfer:
		return 1, nil
	default:
		return 0, fmt.Errorf("NGAP message type (%s) not supported", msgType)
	}
}
