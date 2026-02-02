package ngap

import "github.com/free5gc/ngap/ngapType"

func getMessageType(pdu *ngapType.NGAPPDU) string {
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		if pdu.InitiatingMessage != nil {
			return getInitiatingMessageType(pdu.InitiatingMessage.Value.Present)
		}
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		if pdu.SuccessfulOutcome != nil {
			return getSuccessfulOutcomeMessageType(pdu.SuccessfulOutcome.Value.Present)
		}
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		if pdu.UnsuccessfulOutcome != nil {
			return getUnsuccessfulOutcomeMessageType(pdu.UnsuccessfulOutcome.Value.Present)
		}
	default:
		return "UnknownMessage"
	}

	return "UnknownMessage"
}

func getInitiatingMessageType(present int) string {
	switch present {
	case ngapType.InitiatingMessagePresentNothing:
		return "Nothing"
	case ngapType.InitiatingMessagePresentAMFConfigurationUpdate:
		return "AMFConfigurationUpdate"
	case ngapType.InitiatingMessagePresentHandoverCancel:
		return "HandoverCancel"
	case ngapType.InitiatingMessagePresentHandoverRequired:
		return "HandoverRequired"
	case ngapType.InitiatingMessagePresentHandoverRequest:
		return "HandoverRequest"
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		return "InitialContextSetupRequest"
	case ngapType.InitiatingMessagePresentNGReset:
		return "NGReset"
	case ngapType.InitiatingMessagePresentNGSetupRequest:
		return "NGSetupRequest"
	case ngapType.InitiatingMessagePresentPathSwitchRequest:
		return "PathSwitchRequest"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyRequest:
		return "PDUSessionResourceModifyRequest"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyIndication:
		return "PDUSessionResourceModifyIndication"
	case ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand:
		return "PDUSessionResourceReleaseCommand"
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		return "PDUSessionResourceSetupRequest"
	case ngapType.InitiatingMessagePresentPWSCancelRequest:
		return "PWSCancelRequest"
	case ngapType.InitiatingMessagePresentRANConfigurationUpdate:
		return "RANConfigurationUpdate"
	case ngapType.InitiatingMessagePresentUEContextModificationRequest:
		return "UEContextModificationRequest"
	case ngapType.InitiatingMessagePresentUEContextReleaseCommand:
		return "UEContextReleaseCommand"
	case ngapType.InitiatingMessagePresentUERadioCapabilityCheckRequest:
		return "UERadioCapabilityCheckRequest"
	case ngapType.InitiatingMessagePresentWriteReplaceWarningRequest:
		return "WriteReplaceWarningRequest"
	case ngapType.InitiatingMessagePresentAMFStatusIndication:
		return "AMFStatusIndication"
	case ngapType.InitiatingMessagePresentCellTrafficTrace:
		return "CellTrafficTrace"
	case ngapType.InitiatingMessagePresentDeactivateTrace:
		return "DeactivateTrace"
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		return "DownlinkNASTransport"
	case ngapType.InitiatingMessagePresentDownlinkNonUEAssociatedNRPPaTransport:
		return "DownlinkNonUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentDownlinkRANConfigurationTransfer:
		return "DownlinkRANConfigurationTransfer"
	case ngapType.InitiatingMessagePresentDownlinkRANStatusTransfer:
		return "DownlinkRANStatusTransfer"
	case ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport:
		return "DownlinkUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentErrorIndication:
		return "ErrorIndication"
	case ngapType.InitiatingMessagePresentHandoverNotify:
		return "HandoverNotify"
	case ngapType.InitiatingMessagePresentInitialUEMessage:
		return "InitialUEMessage"
	case ngapType.InitiatingMessagePresentLocationReport:
		return "LocationReport"
	case ngapType.InitiatingMessagePresentLocationReportingControl:
		return "LocationReportingControl"
	case ngapType.InitiatingMessagePresentLocationReportingFailureIndication:
		return "LocationReportingFailureIndication"
	case ngapType.InitiatingMessagePresentNASNonDeliveryIndication:
		return "NASNonDeliveryIndication"
	case ngapType.InitiatingMessagePresentOverloadStart:
		return "OverloadStart"
	case ngapType.InitiatingMessagePresentOverloadStop:
		return "OverloadStop"
	case ngapType.InitiatingMessagePresentPaging:
		return "Paging"
	case ngapType.InitiatingMessagePresentPDUSessionResourceNotify:
		return "PDUSessionResourceNotify"
	case ngapType.InitiatingMessagePresentPrivateMessage:
		return "PrivateMessage"
	case ngapType.InitiatingMessagePresentPWSFailureIndication:
		return "PWSFailureIndication"
	case ngapType.InitiatingMessagePresentPWSRestartIndication:
		return "PWSRestartIndication"
	case ngapType.InitiatingMessagePresentRerouteNASRequest:
		return "RerouteNASRequest"
	case ngapType.InitiatingMessagePresentRRCInactiveTransitionReport:
		return "RRCInactiveTransitionReport"
	case ngapType.InitiatingMessagePresentSecondaryRATDataUsageReport:
		return "SecondaryRATDataUsageReport"
	case ngapType.InitiatingMessagePresentTraceFailureIndication:
		return "TraceFailureIndication"
	case ngapType.InitiatingMessagePresentTraceStart:
		return "TraceStart"
	case ngapType.InitiatingMessagePresentUEContextReleaseRequest:
		return "UEContextReleaseRequest"
	case ngapType.InitiatingMessagePresentUERadioCapabilityInfoIndication:
		return "UERadioCapabilityInfoIndication"
	case ngapType.InitiatingMessagePresentUETNLABindingReleaseRequest:
		return "UETNLABindingReleaseRequest"
	case ngapType.InitiatingMessagePresentUplinkNASTransport:
		return "UplinkNASTransport"
	case ngapType.InitiatingMessagePresentUplinkNonUEAssociatedNRPPaTransport:
		return "UplinkNonUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentUplinkRANConfigurationTransfer:
		return "UplinkRANConfigurationTransfer"
	case ngapType.InitiatingMessagePresentUplinkRANStatusTransfer:
		return "UplinkRANStatusTransfer"
	case ngapType.InitiatingMessagePresentUplinkUEAssociatedNRPPaTransport:
		return "UplinkUEAssociatedNRPPaTransport"
	default:
		return "UnknownMessage"
	}
}

func getSuccessfulOutcomeMessageType(present int) string {
	switch present {
	case ngapType.SuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.SuccessfulOutcomePresentAMFConfigurationUpdateAcknowledge:
		return "AMFConfigurationUpdateAcknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCancelAcknowledge:
		return "HandoverCancelAcknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCommand:
		return "HandoverCommand"
	case ngapType.SuccessfulOutcomePresentHandoverRequestAcknowledge:
		return "HandoverRequestAcknowledge"
	case ngapType.SuccessfulOutcomePresentInitialContextSetupResponse:
		return "InitialContextSetupResponse"
	case ngapType.SuccessfulOutcomePresentNGResetAcknowledge:
		return "NGResetAcknowledge"
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		return "NGSetupResponse"
	case ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge:
		return "PathSwitchRequestAcknowledge"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyResponse:
		return "PDUSessionResourceModifyResponse"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyConfirm:
		return "PDUSessionResourceModifyConfirm"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceReleaseResponse:
		return "PDUSessionResourceReleaseResponse"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceSetupResponse:
		return "PDUSessionResourceSetupResponse"
	case ngapType.SuccessfulOutcomePresentPWSCancelResponse:
		return "PWSCancelResponse"
	case ngapType.SuccessfulOutcomePresentRANConfigurationUpdateAcknowledge:
		return "RANConfigurationUpdateAcknowledge"
	case ngapType.SuccessfulOutcomePresentUEContextModificationResponse:
		return "UEContextModificationResponse"
	case ngapType.SuccessfulOutcomePresentUEContextReleaseComplete:
		return "UEContextReleaseComplete"
	case ngapType.SuccessfulOutcomePresentUERadioCapabilityCheckResponse:
		return "UERadioCapabilityCheckResponse"
	case ngapType.SuccessfulOutcomePresentWriteReplaceWarningResponse:
		return "WriteReplaceWarningResponse"
	default:
		return "Unknown"
	}
}

func getUnsuccessfulOutcomeMessageType(present int) string {
	switch present {
	case ngapType.UnsuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.UnsuccessfulOutcomePresentAMFConfigurationUpdateFailure:
		return "AMFConfigurationUpdateFailure"
	case ngapType.UnsuccessfulOutcomePresentHandoverPreparationFailure:
		return "HandoverPreparationFailure"
	case ngapType.UnsuccessfulOutcomePresentHandoverFailure:
		return "HandoverFailure"
	case ngapType.UnsuccessfulOutcomePresentInitialContextSetupFailure:
		return "InitialContextSetupFailure"
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		return "NGSetupFailure"
	case ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure:
		return "PathSwitchRequestFailure"
	case ngapType.UnsuccessfulOutcomePresentRANConfigurationUpdateFailure:
		return "RANConfigurationUpdateFailure"
	case ngapType.UnsuccessfulOutcomePresentUEContextModificationFailure:
		return "UEContextModificationFailure"
	default:
		return "Unknown"
	}
}

func getPDUCategory(present int) string {
	switch present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		return "InitiatingMessage"
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		return "SuccessfulOutcome"
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		return "UnsuccessfulOutcome"
	default:
		return "Unknown"
	}
}
