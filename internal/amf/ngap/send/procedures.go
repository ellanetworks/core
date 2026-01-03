package send

import "fmt"

type NGAPProcedure string

const (
	// Non-UE associated NGAP procedures
	NGAPProcedureNGSetupResponse                   NGAPProcedure = "NGSetupResponse"
	NGAPProcedureNGSetupFailure                    NGAPProcedure = "NGSetupFailure"
	NGAPProcedurePaging                            NGAPProcedure = "Paging"
	NGAPProcedureNGResetAcknowledge                NGAPProcedure = "NGResetAcknowledge"
	NGAPProcedureErrorIndication                   NGAPProcedure = "ErrorIndication"
	NGAPProcedureRanConfigurationUpdateAcknowledge NGAPProcedure = "RANConfigurationUpdateAcknowledge"
	NGAPProcedureRanConfigurationUpdateFailure     NGAPProcedure = "RANConfigurationUpdateFailure"
	NGAPProcedureAMFStatusIndication               NGAPProcedure = "AMFStatusIndication"
	NGAPProcedureDownlinkRanConfigurationTransfer  NGAPProcedure = "DownlinkRANConfigurationTransfer"

	// UE-associated NGAP procedures
	NGAPProcedureInitialContextSetupRequest       NGAPProcedure = "InitialContextSetupRequest"
	NGAPProcedurePDUSessionResourceModifyRequest  NGAPProcedure = "PDUSessionResourceModifyRequest"
	NGAPProcedurePDUSessionResourceModifyConfirm  NGAPProcedure = "PDUSessionResourceModifyConfirm"
	NGAPProcedurePDUSessionResourceSetupRequest   NGAPProcedure = "PDUSessionResourceSetupRequest"
	NGAPProcedurePDUSessionResourceReleaseCommand NGAPProcedure = "PDUSessionResourceReleaseCommand"
	NGAPProcedureDownlinkNasTransport             NGAPProcedure = "DownlinkNasTransport"
	NGAPProcedureLocationReportingControl         NGAPProcedure = "LocationReportingControl"
	NGAPProcedurePathSwitchRequestFailure         NGAPProcedure = "PathSwitchRequestFailure"
	NGAPProcedurePathSwitchRequestAcknowledge     NGAPProcedure = "PathSwitchRequestAcknowledge"
	NGAPProcedureHandoverRequest                  NGAPProcedure = "HandoverRequest"
	NGAPProcedureHandoverCommand                  NGAPProcedure = "HandoverCommand"
	NGAPProcedureHandoverCancelAcknowledge        NGAPProcedure = "HandoverCancelAcknowledge"
	NGAPProcedureHandoverPreparationFailure       NGAPProcedure = "HandoverPreparationFailure"
	NGAPProcedureUEContextReleaseCommand          NGAPProcedure = "UEContextReleaseCommand"
)

func getSCTPStreamID(msgType NGAPProcedure) (uint16, error) {
	switch msgType {
	// Non-UE procedures
	case NGAPProcedureNGSetupResponse, NGAPProcedureNGSetupFailure,
		NGAPProcedurePaging, NGAPProcedureNGResetAcknowledge,
		NGAPProcedureErrorIndication, NGAPProcedureRanConfigurationUpdateAcknowledge,
		NGAPProcedureRanConfigurationUpdateFailure, NGAPProcedureAMFStatusIndication,
		NGAPProcedureDownlinkRanConfigurationTransfer:
		return 0, nil

	// UE-associated procedures
	case NGAPProcedureInitialContextSetupRequest, NGAPProcedureUEContextReleaseCommand,
		NGAPProcedureDownlinkNasTransport, NGAPProcedurePDUSessionResourceSetupRequest,
		NGAPProcedurePDUSessionResourceReleaseCommand, NGAPProcedureHandoverRequest,
		NGAPProcedureHandoverCommand, NGAPProcedureHandoverPreparationFailure,
		NGAPProcedurePathSwitchRequestAcknowledge, NGAPProcedurePDUSessionResourceModifyRequest,
		NGAPProcedurePDUSessionResourceModifyConfirm, NGAPProcedureHandoverCancelAcknowledge,
		NGAPProcedureLocationReportingControl, NGAPProcedurePathSwitchRequestFailure:
		return 1, nil
	default:
		return 0, fmt.Errorf("NGAP message type (%s) not supported", msgType)
	}
}
