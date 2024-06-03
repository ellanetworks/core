package ngapmsgtypes

import (
	"github.com/omec-project/ngap/ngapType"
)

var NgapMsg map[int64]string

func init() {
	BuildProcedureCodeToMsgMap()
}

func BuildProcedureCodeToMsgMap() {
	NgapMsg = make(map[int64]string, 255)
	NgapMsg[ngapType.ProcedureCodeAMFConfigurationUpdate] = "AMFConfigurationUpdate"
	NgapMsg[ngapType.ProcedureCodeAMFStatusIndication] = "AMFStatusIndication"
	NgapMsg[ngapType.ProcedureCodeCellTrafficTrace] = "CellTrafficTrace"
	NgapMsg[ngapType.ProcedureCodeDeactivateTrace] = "DeactivateTrace"
	NgapMsg[ngapType.ProcedureCodeDownlinkNASTransport] = "DownlinkNASTransport"
	NgapMsg[ngapType.ProcedureCodeDownlinkNonUEAssociatedNRPPaTransport] = "DownlinkNonUEAssociatedNRPPaTransport"
	NgapMsg[ngapType.ProcedureCodeDownlinkRANConfigurationTransfer] = "DownlinkRANConfigurationTransfer"
	NgapMsg[ngapType.ProcedureCodeDownlinkRANStatusTransfer] = "DownlinkRANStatusTransfer"
	NgapMsg[ngapType.ProcedureCodeDownlinkUEAssociatedNRPPaTransport] = "DownlinkUEAssociatedNRPPaTransport"
	NgapMsg[ngapType.ProcedureCodeErrorIndication] = "ErrorIndication"
	NgapMsg[ngapType.ProcedureCodeHandoverCancel] = "HandoverCancel"
	NgapMsg[ngapType.ProcedureCodeHandoverNotification] = "HandoverNotification"
	NgapMsg[ngapType.ProcedureCodeHandoverPreparation] = "HandoverPreparation"
	NgapMsg[ngapType.ProcedureCodeHandoverResourceAllocation] = "HandoverResourceAllocation"
	NgapMsg[ngapType.ProcedureCodeInitialContextSetup] = "InitialContextSetup"
	NgapMsg[ngapType.ProcedureCodeInitialUEMessage] = "InitialUEMessage"
	NgapMsg[ngapType.ProcedureCodeLocationReportingControl] = "LocationReportingControl"
	NgapMsg[ngapType.ProcedureCodeLocationReportingFailureIndication] = "LocationReportingFailureIndication"
	NgapMsg[ngapType.ProcedureCodeLocationReport] = "LocationReport"
	NgapMsg[ngapType.ProcedureCodeNASNonDeliveryIndication] = "NASNonDeliveryIndication"
	NgapMsg[ngapType.ProcedureCodeNGReset] = "NGReset"
	NgapMsg[ngapType.ProcedureCodeNGSetup] = "NGSetup"
	NgapMsg[ngapType.ProcedureCodeOverloadStart] = "OverloadStart"
	NgapMsg[ngapType.ProcedureCodeOverloadStop] = "OverloadStop"
	NgapMsg[ngapType.ProcedureCodePaging] = "Paging"
	NgapMsg[ngapType.ProcedureCodePathSwitchRequest] = "PathSwitchRequest"
	NgapMsg[ngapType.ProcedureCodePDUSessionResourceModify] = "PDUSessionResourceModify"
	NgapMsg[ngapType.ProcedureCodePDUSessionResourceModifyIndication] = "PDUSessionResourceModifyIndication"
	NgapMsg[ngapType.ProcedureCodePDUSessionResourceRelease] = "PDUSessionResourceRelease"
	NgapMsg[ngapType.ProcedureCodePDUSessionResourceSetup] = "PDUSessionResourceSetup"
	NgapMsg[ngapType.ProcedureCodePDUSessionResourceNotify] = "PDUSessionResourceNotify"
	NgapMsg[ngapType.ProcedureCodePrivateMessage] = "PrivateMessage"
	NgapMsg[ngapType.ProcedureCodePWSCancel] = "PWSCancel"
	NgapMsg[ngapType.ProcedureCodePWSFailureIndication] = "PWSFailureIndication"
	NgapMsg[ngapType.ProcedureCodePWSRestartIndication] = "PWSRestartIndication"
	NgapMsg[ngapType.ProcedureCodeRANConfigurationUpdate] = "RANConfigurationUpdate"
	NgapMsg[ngapType.ProcedureCodeRerouteNASRequest] = "RerouteNASRequest"
	NgapMsg[ngapType.ProcedureCodeRRCInactiveTransitionReport] = "RRCInactiveTransitionReport"
	NgapMsg[ngapType.ProcedureCodeTraceFailureIndication] = "TraceFailureIndication"
	NgapMsg[ngapType.ProcedureCodeTraceStart] = "TraceStart"
	NgapMsg[ngapType.ProcedureCodeUEContextModification] = "UEContextModification"
	NgapMsg[ngapType.ProcedureCodeUEContextRelease] = "UEContextRelease"
	NgapMsg[ngapType.ProcedureCodeUEContextReleaseRequest] = "UEContextReleaseRequest"
	NgapMsg[ngapType.ProcedureCodeUERadioCapabilityCheck] = "UERadioCapabilityCheck"
	NgapMsg[ngapType.ProcedureCodeUERadioCapabilityInfoIndication] = "UERadioCapabilityInfoIndication"
	NgapMsg[ngapType.ProcedureCodeUETNLABindingRelease] = "UETNLABindingRelease"
	NgapMsg[ngapType.ProcedureCodeUplinkNASTransport] = "UplinkNASTransport"
	NgapMsg[ngapType.ProcedureCodeUplinkNonUEAssociatedNRPPaTransport] = "NonUEAssociatedNRPPaTransport"
	NgapMsg[ngapType.ProcedureCodeUplinkRANConfigurationTransfer] = "RANConfigurationTransfer"
	NgapMsg[ngapType.ProcedureCodeUplinkRANStatusTransfer] = "RANStatusTransfer"
	NgapMsg[ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport] = "UplinkUEAssociatedNRPPaTransport"
	NgapMsg[ngapType.ProcedureCodeWriteReplaceWarning] = "WriteReplaceWarning"
	NgapMsg[ngapType.ProcedureCodeSecondaryRATDataUsageReport] = "SecondaryRATDataUsageReport"
}
