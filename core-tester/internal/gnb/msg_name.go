package gnb

import "github.com/free5gc/ngap/ngapType"

func getMessageName(pduType int, msgType int) string {
	switch pduType {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		return getInitiatingMessageName(msgType)
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		return getSuccessfulOutcomeName(msgType)
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		return getUnsuccessfulOutcomeName(msgType)
	default:
		return "Unknown"
	}
}

func getInitiatingMessageName(msgType int) string {
	switch msgType {
	case ngapType.InitiatingMessagePresentNothing:
		return "Nothing"
	case ngapType.InitiatingMessagePresentAMFConfigurationUpdate:
		return "AMF Configuration Update"
	case ngapType.InitiatingMessagePresentHandoverCancel:
		return "Handover Cancel"
	case ngapType.InitiatingMessagePresentHandoverRequired:
		return "Handover Required"
	case ngapType.InitiatingMessagePresentHandoverRequest:
		return "Handover Request"
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		return "Initial Context Setup Request"
	case ngapType.InitiatingMessagePresentNGReset:
		return "NG Reset"
	case ngapType.InitiatingMessagePresentNGSetupRequest:
		return "NG Setup Request"
	case ngapType.InitiatingMessagePresentPathSwitchRequest:
		return "Path Switch Request"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyRequest:
		return "PDU Session Resource Modify Request"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyIndication:
		return "PDU Session Resource Modify Indication"
	case ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand:
		return "PDU Session Resource Release Command"
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		return "PDU Session Resource Setup Request"
	case ngapType.InitiatingMessagePresentPWSCancelRequest:
		return "PWS Cancel Request"
	case ngapType.InitiatingMessagePresentRANConfigurationUpdate:
		return "RAN Configuration Update"
	case ngapType.InitiatingMessagePresentUEContextModificationRequest:
		return "UE Context Modification Request"
	case ngapType.InitiatingMessagePresentUEContextReleaseCommand:
		return "UE Context Release Command"
	case ngapType.InitiatingMessagePresentUERadioCapabilityCheckRequest:
		return "UE Radio Capability Check Request"
	case ngapType.InitiatingMessagePresentWriteReplaceWarningRequest:
		return "Write Replace Warning Request"
	case ngapType.InitiatingMessagePresentAMFStatusIndication:
		return "AMF Status Indication"
	case ngapType.InitiatingMessagePresentCellTrafficTrace:
		return "Cell Traffic Trace"
	case ngapType.InitiatingMessagePresentDeactivateTrace:
		return "Deactivate Trace"
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		return "Downlink NAS Transport"
	case ngapType.InitiatingMessagePresentDownlinkNonUEAssociatedNRPPaTransport:
		return "Downlink Non-UE Associated NRPPa Transport"
	case ngapType.InitiatingMessagePresentDownlinkRANConfigurationTransfer:
		return "Downlink RAN Configuration Transfer"
	case ngapType.InitiatingMessagePresentDownlinkRANStatusTransfer:
		return "Downlink RAN Status Transfer"
	case ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport:
		return "Downlink UE Associated NRPPa Transport"
	case ngapType.InitiatingMessagePresentErrorIndication:
		return "Error Indication"
	case ngapType.InitiatingMessagePresentHandoverNotify:
		return "Handover Notify"
	case ngapType.InitiatingMessagePresentInitialUEMessage:
		return "Initial UE Message"
	case ngapType.InitiatingMessagePresentLocationReport:
		return "Location Report"
	case ngapType.InitiatingMessagePresentLocationReportingControl:
		return "Location Reporting Control"
	case ngapType.InitiatingMessagePresentLocationReportingFailureIndication:
		return "Location Reporting Failure Indication"
	case ngapType.InitiatingMessagePresentNASNonDeliveryIndication:
		return "NAS Non-Delivery Indication"
	case ngapType.InitiatingMessagePresentOverloadStart:
		return "Overload Start"
	case ngapType.InitiatingMessagePresentOverloadStop:
		return "Overload Stop"
	case ngapType.InitiatingMessagePresentPaging:
		return "Paging"
	case ngapType.InitiatingMessagePresentPDUSessionResourceNotify:
		return "PDU Session Resource Notify"
	case ngapType.InitiatingMessagePresentPrivateMessage:
		return "Private Message"
	case ngapType.InitiatingMessagePresentPWSFailureIndication:
		return "PWS Failure Indication"
	case ngapType.InitiatingMessagePresentPWSRestartIndication:
		return "PWS Restart Indication"
	case ngapType.InitiatingMessagePresentRerouteNASRequest:
		return "Reroute NAS Request"
	case ngapType.InitiatingMessagePresentRRCInactiveTransitionReport:
		return "RRC Inactive Transition Report"
	case ngapType.InitiatingMessagePresentSecondaryRATDataUsageReport:
		return "Secondary RAT Data Usage Report"
	case ngapType.InitiatingMessagePresentTraceFailureIndication:
		return "Trace Failure Indication"
	case ngapType.InitiatingMessagePresentTraceStart:
		return "Trace Start"
	case ngapType.InitiatingMessagePresentUEContextReleaseRequest:
		return "UE Context Release Request"
	case ngapType.InitiatingMessagePresentUERadioCapabilityInfoIndication:
		return "UE Radio Capability Info Indication"
	case ngapType.InitiatingMessagePresentUETNLABindingReleaseRequest:
		return "UE TNLA Binding Release Request"
	case ngapType.InitiatingMessagePresentUplinkNASTransport:
		return "Uplink NAS Transport"
	case ngapType.InitiatingMessagePresentUplinkNonUEAssociatedNRPPaTransport:
		return "Uplink Non-UE Associated NRPPa Transport"
	case ngapType.InitiatingMessagePresentUplinkRANConfigurationTransfer:
		return "Uplink RAN Configuration Transfer"
	case ngapType.InitiatingMessagePresentUplinkRANStatusTransfer:
		return "Uplink RAN Status Transfer"
	case ngapType.InitiatingMessagePresentUplinkUEAssociatedNRPPaTransport:
		return "Uplink UE Associated NRPPa Transport"
	default:
		return "Unknown"
	}
}

func getSuccessfulOutcomeName(msgType int) string {
	switch msgType {
	case ngapType.SuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.SuccessfulOutcomePresentAMFConfigurationUpdateAcknowledge:
		return "AMF Configuration Update Acknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCancelAcknowledge:
		return "Handover Cancel Acknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCommand:
		return "Handover Command"
	case ngapType.SuccessfulOutcomePresentHandoverRequestAcknowledge:
		return "Handover Request Acknowledge"
	case ngapType.SuccessfulOutcomePresentInitialContextSetupResponse:
		return "Initial Context Setup Response"
	case ngapType.SuccessfulOutcomePresentNGResetAcknowledge:
		return "NG Reset Acknowledge"
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		return "NG Setup Response"
	case ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge:
		return "Path Switch Request Acknowledge"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyResponse:
		return "PDU Session Resource Modify Response"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyConfirm:
		return "PDU Session Resource Modify Confirm"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceReleaseResponse:
		return "PDU Session Resource Release Response"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceSetupResponse:
		return "PDU Session Resource Setup Response"
	case ngapType.SuccessfulOutcomePresentPWSCancelResponse:
		return "PWS Cancel Response"
	case ngapType.SuccessfulOutcomePresentRANConfigurationUpdateAcknowledge:
		return "RAN Configuration Update Acknowledge"
	case ngapType.SuccessfulOutcomePresentUEContextModificationResponse:
		return "UE Context Modification Response"
	case ngapType.SuccessfulOutcomePresentUEContextReleaseComplete:
		return "UE Context Release Complete"
	case ngapType.SuccessfulOutcomePresentUERadioCapabilityCheckResponse:
		return "UE Radio Capability Check Response"
	case ngapType.SuccessfulOutcomePresentWriteReplaceWarningResponse:
		return "Write Replace Warning Response"
	default:
		return "Unknown"
	}
}

func getUnsuccessfulOutcomeName(msgType int) string {
	switch msgType {
	case ngapType.UnsuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.UnsuccessfulOutcomePresentAMFConfigurationUpdateFailure:
		return "AMF Configuration Update Failure"
	case ngapType.UnsuccessfulOutcomePresentHandoverPreparationFailure:
		return "Handover Preparation Failure"
	case ngapType.UnsuccessfulOutcomePresentHandoverFailure:
		return "Handover Failure"
	case ngapType.UnsuccessfulOutcomePresentInitialContextSetupFailure:
		return "Initial Context Setup Failure"
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		return "NG Setup Failure"
	case ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure:
		return "Path Switch Request Failure"
	case ngapType.UnsuccessfulOutcomePresentRANConfigurationUpdateFailure:
		return "RAN Configuration Update Failure"
	case ngapType.UnsuccessfulOutcomePresentUEContextModificationFailure:
		return "UE Context Modification Failure"
	default:
		return "Unknown"
	}
}
