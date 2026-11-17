// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "reflect"

type messageCase struct {
	asn1  string
	rows  []tableRowSpec
	typ   reflect.Type
	parse func([]byte) (any, error)
}

func messageCases() []messageCase {
	return []messageCase{
		{"DownlinkNASTransport", tableRows(downlinkNASTransportIEs), reflect.TypeFor[DownlinkNASTransport](), func(b []byte) (any, error) { return ParseDownlinkNASTransport(b) }},
		{"DownlinkNonUEAssociatedLPPaTransport", tableRows(downlinkNonUEAssociatedLPPaTransportIEs), reflect.TypeFor[DownlinkNonUEAssociatedLPPaTransport](), func(b []byte) (any, error) { return ParseDownlinkNonUEAssociatedLPPaTransport(b) }},
		{"DownlinkUEAssociatedLPPaTransport", tableRows(downlinkUEAssociatedLPPaTransportIEs), reflect.TypeFor[DownlinkUEAssociatedLPPaTransport](), func(b []byte) (any, error) { return ParseDownlinkUEAssociatedLPPaTransport(b) }},
		{"E-RABModificationConfirm", tableRows(erabModificationConfirmIEs), reflect.TypeFor[ERABModificationConfirm](), func(b []byte) (any, error) { return ParseERABModificationConfirm(b) }},
		{"E-RABModificationIndication", tableRows(eRABModificationIndicationIEs), reflect.TypeFor[ERABModificationIndication](), func(b []byte) (any, error) { return ParseERABModificationIndication(b) }},
		{"E-RABModifyRequest", tableRows(eRABModifyRequestIEs), reflect.TypeFor[ERABModifyRequest](), func(b []byte) (any, error) { return ParseERABModifyRequest(b) }},
		{"E-RABModifyResponse", tableRows(eRABModifyResponseIEs), reflect.TypeFor[ERABModifyResponse](), func(b []byte) (any, error) { return ParseERABModifyResponse(b) }},
		{"E-RABReleaseCommand", tableRows(eRABReleaseCommandIEs), reflect.TypeFor[ERABReleaseCommand](), func(b []byte) (any, error) { return ParseERABReleaseCommand(b) }},
		{"E-RABReleaseIndication", tableRows(eRABReleaseIndicationIEs), reflect.TypeFor[ERABReleaseIndication](), func(b []byte) (any, error) { return ParseERABReleaseIndication(b) }},
		{"E-RABReleaseResponse", tableRows(eRABReleaseResponseIEs), reflect.TypeFor[ERABReleaseResponse](), func(b []byte) (any, error) { return ParseERABReleaseResponse(b) }},
		{"E-RABSetupRequest", tableRows(eRABSetupRequestIEs), reflect.TypeFor[ERABSetupRequest](), func(b []byte) (any, error) { return ParseERABSetupRequest(b) }},
		{"E-RABSetupResponse", tableRows(eRABSetupResponseIEs), reflect.TypeFor[ERABSetupResponse](), func(b []byte) (any, error) { return ParseERABSetupResponse(b) }},
		{"ENBConfigurationTransfer", tableRows(eNBConfigurationTransferIEs), reflect.TypeFor[ENBConfigurationTransfer](), func(b []byte) (any, error) { return ParseENBConfigurationTransfer(b) }},
		{"ENBConfigurationUpdate", tableRows(eNBConfigurationUpdateIEs), reflect.TypeFor[ENBConfigurationUpdate](), func(b []byte) (any, error) { return ParseENBConfigurationUpdate(b) }},
		{"ENBConfigurationUpdateAcknowledge", tableRows(eNBConfigurationUpdateAcknowledgeIEs), reflect.TypeFor[ENBConfigurationUpdateAcknowledge](), func(b []byte) (any, error) { return ParseENBConfigurationUpdateAcknowledge(b) }},
		{"ENBConfigurationUpdateFailure", tableRows(eNBConfigurationUpdateFailureIEs), reflect.TypeFor[ENBConfigurationUpdateFailure](), func(b []byte) (any, error) { return ParseENBConfigurationUpdateFailure(b) }},
		{"ENBStatusTransfer", tableRows(eNBStatusTransferIEs), reflect.TypeFor[ENBStatusTransfer](), func(b []byte) (any, error) { return ParseENBStatusTransfer(b) }},
		{"ErrorIndication", tableRows(errorIndicationIEs), reflect.TypeFor[ErrorIndication](), func(b []byte) (any, error) { return ParseErrorIndication(b) }},
		{"HandoverCancel", tableRows(handoverCancelIEs), reflect.TypeFor[HandoverCancel](), func(b []byte) (any, error) { return ParseHandoverCancel(b) }},
		{"HandoverCancelAcknowledge", tableRows(handoverCancelAcknowledgeIEs), reflect.TypeFor[HandoverCancelAcknowledge](), func(b []byte) (any, error) { return ParseHandoverCancelAcknowledge(b) }},
		{"HandoverCommand", tableRows(handoverCommandIEs), reflect.TypeFor[HandoverCommand](), func(b []byte) (any, error) { return ParseHandoverCommand(b) }},
		{"HandoverFailure", tableRows(handoverFailureIEs), reflect.TypeFor[HandoverFailure](), func(b []byte) (any, error) { return ParseHandoverFailure(b) }},
		{"HandoverNotify", tableRows(handoverNotifyIEs), reflect.TypeFor[HandoverNotify](), func(b []byte) (any, error) { return ParseHandoverNotify(b) }},
		{"HandoverPreparationFailure", tableRows(handoverPreparationFailureIEs), reflect.TypeFor[HandoverPreparationFailure](), func(b []byte) (any, error) { return ParseHandoverPreparationFailure(b) }},
		{"HandoverRequest", tableRows(handoverRequestIEs), reflect.TypeFor[HandoverRequest](), func(b []byte) (any, error) { return ParseHandoverRequest(b) }},
		{"HandoverRequestAcknowledge", tableRows(handoverRequestAcknowledgeIEs), reflect.TypeFor[HandoverRequestAcknowledge](), func(b []byte) (any, error) { return ParseHandoverRequestAcknowledge(b) }},
		{"HandoverRequired", tableRows(handoverRequiredIEs), reflect.TypeFor[HandoverRequired](), func(b []byte) (any, error) { return ParseHandoverRequired(b) }},
		{"InitialContextSetupFailure", tableRows(initialContextSetupFailureIEs), reflect.TypeFor[InitialContextSetupFailure](), func(b []byte) (any, error) { return ParseInitialContextSetupFailure(b) }},
		{"InitialContextSetupRequest", tableRows(initialContextSetupRequestIEs), reflect.TypeFor[InitialContextSetupRequest](), func(b []byte) (any, error) { return ParseInitialContextSetupRequest(b) }},
		{"InitialContextSetupResponse", tableRows(initialContextSetupResponseIEs), reflect.TypeFor[InitialContextSetupResponse](), func(b []byte) (any, error) { return ParseInitialContextSetupResponse(b) }},
		{"InitialUEMessage", tableRows(initialUEMessageIEs), reflect.TypeFor[InitialUEMessage](), func(b []byte) (any, error) { return ParseInitialUEMessage(b) }},
		{"KillRequest", tableRows(killRequestIEs), reflect.TypeFor[KillRequest](), func(b []byte) (any, error) { return ParseKillRequest(b) }},
		{"KillResponse", tableRows(killResponseIEs), reflect.TypeFor[KillResponse](), func(b []byte) (any, error) { return ParseKillResponse(b) }},
		{"LocationReport", tableRows(locationReportIEs), reflect.TypeFor[LocationReport](), func(b []byte) (any, error) { return ParseLocationReport(b) }},
		{"MMEConfigurationTransfer", tableRows(mMEConfigurationTransferIEs), reflect.TypeFor[MMEConfigurationTransfer](), func(b []byte) (any, error) { return ParseMMEConfigurationTransfer(b) }},
		{"MMEConfigurationUpdate", tableRows(mMEConfigurationUpdateIEs), reflect.TypeFor[MMEConfigurationUpdate](), func(b []byte) (any, error) { return ParseMMEConfigurationUpdate(b) }},
		{"MMEConfigurationUpdateAcknowledge", tableRows(mMEConfigurationUpdateAcknowledgeIEs), reflect.TypeFor[MMEConfigurationUpdateAcknowledge](), func(b []byte) (any, error) { return ParseMMEConfigurationUpdateAcknowledge(b) }},
		{"MMEConfigurationUpdateFailure", tableRows(mMEConfigurationUpdateFailureIEs), reflect.TypeFor[MMEConfigurationUpdateFailure](), func(b []byte) (any, error) { return ParseMMEConfigurationUpdateFailure(b) }},
		{"MMEStatusTransfer", tableRows(mMEStatusTransferIEs), reflect.TypeFor[MMEStatusTransfer](), func(b []byte) (any, error) { return ParseMMEStatusTransfer(b) }},
		{"NASNonDeliveryIndication", tableRows(nASNonDeliveryIndicationIEs), reflect.TypeFor[NASNonDeliveryIndication](), func(b []byte) (any, error) { return ParseNASNonDeliveryIndication(b) }},
		{"OverloadStart", tableRows(overloadStartIEs), reflect.TypeFor[OverloadStart](), func(b []byte) (any, error) { return ParseOverloadStart(b) }},
		{"OverloadStop", tableRows(overloadStopIEs), reflect.TypeFor[OverloadStop](), func(b []byte) (any, error) { return ParseOverloadStop(b) }},
		{"PWSFailureIndication", tableRows(pWSFailureIndicationIEs), reflect.TypeFor[PWSFailureIndication](), func(b []byte) (any, error) { return ParsePWSFailureIndication(b) }},
		{"PWSRestartIndication", tableRows(pWSRestartIndicationIEs), reflect.TypeFor[PWSRestartIndication](), func(b []byte) (any, error) { return ParsePWSRestartIndication(b) }},
		{"Paging", tableRows(pagingIEs), reflect.TypeFor[Paging](), func(b []byte) (any, error) { return ParsePaging(b) }},
		{"PathSwitchRequest", tableRows(pathSwitchRequestIEs), reflect.TypeFor[PathSwitchRequest](), func(b []byte) (any, error) { return ParsePathSwitchRequest(b) }},
		{"PathSwitchRequestAcknowledge", tableRows(pathSwitchRequestAcknowledgeIEs), reflect.TypeFor[PathSwitchRequestAcknowledge](), func(b []byte) (any, error) { return ParsePathSwitchRequestAcknowledge(b) }},
		{"PathSwitchRequestFailure", tableRows(pathSwitchRequestFailureIEs), reflect.TypeFor[PathSwitchRequestFailure](), func(b []byte) (any, error) { return ParsePathSwitchRequestFailure(b) }},
		{"RerouteNASRequest", tableRows(rerouteNASRequestIEs), reflect.TypeFor[RerouteNASRequest](), func(b []byte) (any, error) { return ParseRerouteNASRequest(b) }},
		{"Reset", tableRows(resetIEs), reflect.TypeFor[Reset](), func(b []byte) (any, error) { return ParseReset(b) }},
		{"ResetAcknowledge", tableRows(resetAcknowledgeIEs), reflect.TypeFor[ResetAcknowledge](), func(b []byte) (any, error) { return ParseResetAcknowledge(b) }},
		{"S1RemovalFailure", tableRows(s1RemovalFailureIEs), reflect.TypeFor[S1RemovalFailure](), func(b []byte) (any, error) { return ParseS1RemovalFailure(b) }},
		{"S1RemovalRequest", tableRows(s1RemovalRequestIEs), reflect.TypeFor[S1RemovalRequest](), func(b []byte) (any, error) { return ParseS1RemovalRequest(b) }},
		{"S1RemovalResponse", tableRows(s1RemovalResponseIEs), reflect.TypeFor[S1RemovalResponse](), func(b []byte) (any, error) { return ParseS1RemovalResponse(b) }},
		{"S1SetupFailure", tableRows(s1SetupFailureIEs), reflect.TypeFor[S1SetupFailure](), func(b []byte) (any, error) { return ParseS1SetupFailure(b) }},
		{"S1SetupRequest", tableRows(s1SetupRequestIEs), reflect.TypeFor[S1SetupRequest](), func(b []byte) (any, error) { return ParseS1SetupRequest(b) }},
		{"S1SetupResponse", tableRows(s1SetupResponseIEs), reflect.TypeFor[S1SetupResponse](), func(b []byte) (any, error) { return ParseS1SetupResponse(b) }},
		{"TraceFailureIndication", tableRows(traceFailureIndicationIEs), reflect.TypeFor[TraceFailureIndication](), func(b []byte) (any, error) { return ParseTraceFailureIndication(b) }},
		{"TraceStart", tableRows(traceStartIEs), reflect.TypeFor[TraceStart](), func(b []byte) (any, error) { return ParseTraceStart(b) }},
		{"UECapabilityInfoIndication", tableRows(uECapabilityInfoIndicationIEs), reflect.TypeFor[UECapabilityInfoIndication](), func(b []byte) (any, error) { return ParseUECapabilityInfoIndication(b) }},
		{"UEContextModificationFailure", tableRows(uEContextModificationFailureIEs), reflect.TypeFor[UEContextModificationFailure](), func(b []byte) (any, error) { return ParseUEContextModificationFailure(b) }},
		{"UEContextModificationRequest", tableRows(uEContextModificationRequestIEs), reflect.TypeFor[UEContextModificationRequest](), func(b []byte) (any, error) { return ParseUEContextModificationRequest(b) }},
		{"UEContextModificationResponse", tableRows(uEContextModificationResponseIEs), reflect.TypeFor[UEContextModificationResponse](), func(b []byte) (any, error) { return ParseUEContextModificationResponse(b) }},
		{"UEContextReleaseCommand", tableRows(uEContextReleaseCommandIEs), reflect.TypeFor[UEContextReleaseCommand](), func(b []byte) (any, error) { return ParseUEContextReleaseCommand(b) }},
		{"UEContextReleaseComplete", tableRows(uEContextReleaseCompleteIEs), reflect.TypeFor[UEContextReleaseComplete](), func(b []byte) (any, error) { return ParseUEContextReleaseComplete(b) }},
		{"UEContextReleaseRequest", tableRows(uEContextReleaseRequestIEs), reflect.TypeFor[UEContextReleaseRequest](), func(b []byte) (any, error) { return ParseUEContextReleaseRequest(b) }},
		{"UplinkNASTransport", tableRows(uplinkNASTransportIEs), reflect.TypeFor[UplinkNASTransport](), func(b []byte) (any, error) { return ParseUplinkNASTransport(b) }},
		{"UplinkNonUEAssociatedLPPaTransport", tableRows(uplinkNonUEAssociatedLPPaTransportIEs), reflect.TypeFor[UplinkNonUEAssociatedLPPaTransport](), func(b []byte) (any, error) { return ParseUplinkNonUEAssociatedLPPaTransport(b) }},
		{"UplinkUEAssociatedLPPaTransport", tableRows(uplinkUEAssociatedLPPaTransportIEs), reflect.TypeFor[UplinkUEAssociatedLPPaTransport](), func(b []byte) (any, error) { return ParseUplinkUEAssociatedLPPaTransport(b) }},
		{"WriteReplaceWarningRequest", tableRows(writeReplaceWarningRequestIEs), reflect.TypeFor[WriteReplaceWarningRequest](), func(b []byte) (any, error) { return ParseWriteReplaceWarningRequest(b) }},
		{"WriteReplaceWarningResponse", tableRows(writeReplaceWarningResponseIEs), reflect.TypeFor[WriteReplaceWarningResponse](), func(b []byte) (any, error) { return ParseWriteReplaceWarningResponse(b) }},
	}
}
