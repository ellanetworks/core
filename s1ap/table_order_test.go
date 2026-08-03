// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"
)

// TestTableOrderMatchesASN1 pins the IEs each table models to their relative
// order in the message's S1AP-PROTOCOL-IES container (TS 36.413 §9.3.3), which
// is the order encode emits them in. IEs a table does not model are not pinned.
func TestTableOrderMatchesASN1(t *testing.T) {
	tests := []struct {
		name string
		got  []ProtocolIEID
		want []ProtocolIEID
	}{
		{"ENBConfigurationTransfer", tableIDs(eNBConfigurationTransferIEs), []ProtocolIEID{idSONConfigurationTransferECT}},
		{"MMEConfigurationTransfer", tableIDs(mMEConfigurationTransferIEs), []ProtocolIEID{idSONConfigurationTransferMCT}},
		{"ENBConfigurationUpdate", tableIDs(eNBConfigurationUpdateIEs), []ProtocolIEID{idENBname, idSupportedTAs, idDefaultPagingDRX}},
		{"ENBConfigurationUpdateAcknowledge", tableIDs(eNBConfigurationUpdateAcknowledgeIEs), []ProtocolIEID{idCriticalityDiagnostics}},
		{"ENBConfigurationUpdateFailure", tableIDs(eNBConfigurationUpdateFailureIEs), []ProtocolIEID{idCause, idTimeToWait, idCriticalityDiagnostics}},
		{"ERABModificationConfirm", tableIDs(erabModificationConfirmIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idERABModifyListBearerModConf, idCriticalityDiagnostics}},
		{"ERABModificationIndication", tableIDs(eRABModificationIndicationIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idERABToBeModifiedListBearerModInd, idERABNotToBeModifiedListBearerModInd, idUserLocationInformation}},
		{"ERABModifyRequest", tableIDs(eRABModifyRequestIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idUEAggregateMaximumBitrate, idERABToBeModifiedListBearerModReq}},
		{"ERABModifyResponse", tableIDs(eRABModifyResponseIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idERABModifyListBearerModRes, idERABFailedToModifyList, idCriticalityDiagnostics, idUserLocationInformation}},
		{"ERABReleaseCommand", tableIDs(eRABReleaseCommandIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idUEAggregateMaximumBitrate, idERABToBeReleasedList, idNASPDU}},
		{"ERABReleaseResponse", tableIDs(eRABReleaseResponseIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idERABReleaseListBearerRelComp, idERABFailedToReleaseList, idCriticalityDiagnostics, idUserLocationInformation}},
		{"ERABSetupRequest", tableIDs(eRABSetupRequestIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idUEAggregateMaximumBitrate, idERABToBeSetupListBearerSUReq}},
		{"ERABSetupResponse", tableIDs(eRABSetupResponseIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idERABSetupListBearerSURes, idERABFailedToSetupListBearerSURes, idCriticalityDiagnostics, idUserLocationInformation}},
		{"ErrorIndication", tableIDs(errorIndicationIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idCause, idCriticalityDiagnostics, idSTMSI}},
		{"HandoverCancel", tableIDs(handoverCancelIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idCause}},
		{"HandoverCancelAcknowledge", tableIDs(handoverCancelAcknowledgeIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID}},
		{"HandoverNotify", tableIDs(handoverNotifyIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idEUTRANCGI, idTAI}},
		{"HandoverRequired", tableIDs(handoverRequiredIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idHandoverType, idCause, idTargetID, idSourceToTargetTransparentContainer}},
		{"HandoverCommand", tableIDs(handoverCommandIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idHandoverType, idERABtoReleaseListHOCmd, idTargetToSourceTransparentContainer}},
		{"HandoverPreparationFailure", tableIDs(handoverPreparationFailureIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idCause, idCriticalityDiagnostics}},
		{"HandoverRequest", tableIDs(handoverRequestIEs), []ProtocolIEID{idMMEUES1APID, idHandoverType, idCause, idUEAggregateMaximumBitrate, idERABToBeSetupListHOReq, idSourceToTargetTransparentContainer, idUESecurityCapabilities, idSecurityContext}},
		{"HandoverRequestAcknowledge", tableIDs(handoverRequestAcknowledgeIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idERABAdmittedList, idERABFailedToSetupListHOReqAck, idTargetToSourceTransparentContainer}},
		{"HandoverFailure", tableIDs(handoverFailureIEs), []ProtocolIEID{idMMEUES1APID, idCause}},
		{"InitialContextSetupRequest", tableIDs(initialContextSetupRequestIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idUEAggregateMaximumBitrate, idERABToBeSetupListCtxtSUReq, idUESecurityCapabilities, idSecurityKey, idUERadioCapability}},
		{"InitialContextSetupResponse", tableIDs(initialContextSetupResponseIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idERABSetupListCtxtSURes, idERABFailedToSetupListCtxtSU, idCriticalityDiagnostics}},
		{"InitialContextSetupFailure", tableIDs(initialContextSetupFailureIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idCause, idCriticalityDiagnostics}},
		{"LocationReport", tableIDs(locationReportIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idEUTRANCGI, idTAI, idRequestType}},
		{"NASNonDeliveryIndication", tableIDs(nASNonDeliveryIndicationIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idNASPDU, idCause}},
		{"InitialUEMessage", tableIDs(initialUEMessageIEs), []ProtocolIEID{idENBUES1APID, idNASPDU, idTAI, idEUTRANCGI, idRRCEstablishmentCause, idSTMSI, idGUMMEI}},
		{"UplinkNASTransport", tableIDs(uplinkNASTransportIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idNASPDU, idEUTRANCGI, idTAI}},
		{"DownlinkNASTransport", tableIDs(downlinkNASTransportIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idNASPDU}},
		{"Paging", tableIDs(pagingIEs), []ProtocolIEID{idUEIdentityIndexValue, idUEPagingID, idPagingDRX, idCNDomain, idTAIList, idPagingPriority, idUERadioCapabilityForPaging}},
		{"PathSwitchRequest", tableIDs(pathSwitchRequestIEs), []ProtocolIEID{idENBUES1APID, idERABToBeSwitchedDLList, idSourceMMEUES1APID, idEUTRANCGI, idTAI, idUESecurityCapabilities}},
		{"PathSwitchRequestAcknowledge", tableIDs(pathSwitchRequestAcknowledgeIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idUEAggregateMaximumBitrate, idERABToBeReleasedList, idSecurityContext, idUESecurityCapabilities}},
		{"PathSwitchRequestFailure", tableIDs(pathSwitchRequestFailureIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idCause, idCriticalityDiagnostics}},
		{"Reset", tableIDs(resetIEs), []ProtocolIEID{idCause, idResetType}},
		{"ResetAcknowledge", tableIDs(resetAcknowledgeIEs), []ProtocolIEID{idUEAssociatedLogicalS1ConnectionListResAck, idCriticalityDiagnostics}},
		{"S1SetupRequest", tableIDs(s1SetupRequestIEs), []ProtocolIEID{idGlobalENBID, idENBname, idSupportedTAs, idDefaultPagingDRX, idUERetentionInformation}},
		{"S1SetupFailure", tableIDs(s1SetupFailureIEs), []ProtocolIEID{idCause, idTimeToWait, idCriticalityDiagnostics}},
		{"S1SetupResponse", tableIDs(s1SetupResponseIEs), []ProtocolIEID{idMMEname, idServedGUMMEIs, idRelativeMMECapacity, idCriticalityDiagnostics, idUERetentionInformation}},
		{"ENBStatusTransfer", tableIDs(eNBStatusTransferIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idENBStatusTransferTransparentContainer}},
		{"MMEStatusTransfer", tableIDs(mMEStatusTransferIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idENBStatusTransferTransparentContainer}},
		{"UECapabilityInfoIndication", tableIDs(uECapabilityInfoIndicationIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idUERadioCapability, idUERadioCapabilityForPaging}},
		{"UEContextReleaseCommand", tableIDs(uEContextReleaseCommandIEs), []ProtocolIEID{idUES1APIDs, idCause}},
		{"UEContextReleaseComplete", tableIDs(uEContextReleaseCompleteIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idCriticalityDiagnostics, idUserLocationInformation}},
		{"UEContextReleaseRequest", tableIDs(uEContextReleaseRequestIEs), []ProtocolIEID{idMMEUES1APID, idENBUES1APID, idCause}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != len(tt.want) {
				t.Fatalf("%d IEs, want %d: %v vs %v", len(tt.got), len(tt.want), tt.got, tt.want)
			}

			for i := range tt.got {
				if tt.got[i] != tt.want[i] {
					t.Errorf("row %d = %s, want %s", i, tt.got[i], tt.want[i])
				}
			}
		})
	}
}

func tableIDs[M any](table []ieSpec[M]) []ProtocolIEID {
	ids := make([]ProtocolIEID, len(table))
	for i, spec := range table {
		ids[i] = spec.id
	}

	return ids
}
