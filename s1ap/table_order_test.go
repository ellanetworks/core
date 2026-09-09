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
		{"ENBConfigurationTransfer", tableIDs(eNBConfigurationTransferIEs), []ProtocolIEID{IDSONConfigurationTransferECT}},
		{"MMEConfigurationTransfer", tableIDs(mMEConfigurationTransferIEs), []ProtocolIEID{IDSONConfigurationTransferMCT}},
		{"ENBConfigurationUpdate", tableIDs(eNBConfigurationUpdateIEs), []ProtocolIEID{IDENBname, IDSupportedTAs, IDDefaultPagingDRX}},
		{"ENBConfigurationUpdateAcknowledge", tableIDs(eNBConfigurationUpdateAcknowledgeIEs), []ProtocolIEID{IDCriticalityDiagnostics}},
		{"ENBConfigurationUpdateFailure", tableIDs(eNBConfigurationUpdateFailureIEs), []ProtocolIEID{IDCause, IDTimeToWait, IDCriticalityDiagnostics}},
		{"MMEConfigurationUpdate", tableIDs(mMEConfigurationUpdateIEs), []ProtocolIEID{IDMMEname, IDServedGUMMEIs, IDRelativeMMECapacity}},
		{"MMEConfigurationUpdateAcknowledge", tableIDs(mMEConfigurationUpdateAcknowledgeIEs), []ProtocolIEID{IDCriticalityDiagnostics}},
		{"MMEConfigurationUpdateFailure", tableIDs(mMEConfigurationUpdateFailureIEs), []ProtocolIEID{IDCause, IDTimeToWait, IDCriticalityDiagnostics}},
		{"ERABModificationConfirm", tableIDs(erabModificationConfirmIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDERABModifyListBearerModConf, IDCriticalityDiagnostics}},
		{"ERABModificationIndication", tableIDs(eRABModificationIndicationIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDERABToBeModifiedListBearerModInd, IDERABNotToBeModifiedListBearerModInd, IDUserLocationInformation}},
		{"ERABModifyRequest", tableIDs(eRABModifyRequestIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDUEAggregateMaximumBitrate, IDERABToBeModifiedListBearerModReq}},
		{"ERABModifyResponse", tableIDs(eRABModifyResponseIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDERABModifyListBearerModRes, IDERABFailedToModifyList, IDCriticalityDiagnostics, IDUserLocationInformation}},
		{"ERABReleaseCommand", tableIDs(eRABReleaseCommandIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDUEAggregateMaximumBitrate, IDERABToBeReleasedList, IDNASPDU}},
		{"ERABReleaseResponse", tableIDs(eRABReleaseResponseIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDERABReleaseListBearerRelComp, IDERABFailedToReleaseList, IDCriticalityDiagnostics, IDUserLocationInformation}},
		{"ERABSetupRequest", tableIDs(eRABSetupRequestIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDUEAggregateMaximumBitrate, IDERABToBeSetupListBearerSUReq}},
		{"ERABSetupResponse", tableIDs(eRABSetupResponseIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDERABSetupListBearerSURes, IDERABFailedToSetupListBearerSURes, IDCriticalityDiagnostics, IDUserLocationInformation}},
		{"ErrorIndication", tableIDs(errorIndicationIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDCause, IDCriticalityDiagnostics, IDSTMSI}},
		{"HandoverCancel", tableIDs(handoverCancelIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDCause}},
		{"HandoverCancelAcknowledge", tableIDs(handoverCancelAcknowledgeIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDCriticalityDiagnostics}},
		{"HandoverNotify", tableIDs(handoverNotifyIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDEUTRANCGI, IDTAI}},
		{"HandoverRequired", tableIDs(handoverRequiredIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDHandoverType, IDCause, IDTargetID, IDDirectForwardingPathAvailability, IDSourceToTargetTransparentContainer}},
		{"HandoverCommand", tableIDs(handoverCommandIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDHandoverType, IDNASSecurityParametersfromEUTRAN, IDERABSubjecttoDataForwardingList, IDERABtoReleaseListHOCmd, IDTargetToSourceTransparentContainer, IDCriticalityDiagnostics}},
		{"HandoverPreparationFailure", tableIDs(handoverPreparationFailureIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDCause, IDCriticalityDiagnostics}},
		{"HandoverRequest", tableIDs(handoverRequestIEs), []ProtocolIEID{IDMMEUES1APID, IDHandoverType, IDCause, IDUEAggregateMaximumBitrate, IDERABToBeSetupListHOReq, IDSourceToTargetTransparentContainer, IDUESecurityCapabilities, IDHandoverRestrictionList, IDSecurityContext, IDNASSecurityParameterstoEUTRAN}},
		{"HandoverRequestAcknowledge", tableIDs(handoverRequestAcknowledgeIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDERABAdmittedList, IDERABFailedToSetupListHOReqAck, IDTargetToSourceTransparentContainer, IDCriticalityDiagnostics}},
		{"HandoverFailure", tableIDs(handoverFailureIEs), []ProtocolIEID{IDMMEUES1APID, IDCause, IDCriticalityDiagnostics}},
		{"InitialContextSetupRequest", tableIDs(initialContextSetupRequestIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDUEAggregateMaximumBitrate, IDERABToBeSetupListCtxtSUReq, IDUESecurityCapabilities, IDSecurityKey, IDUERadioCapability}},
		{"InitialContextSetupResponse", tableIDs(initialContextSetupResponseIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDERABSetupListCtxtSURes, IDERABFailedToSetupListCtxtSURes, IDCriticalityDiagnostics}},
		{"InitialContextSetupFailure", tableIDs(initialContextSetupFailureIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDCause, IDCriticalityDiagnostics}},
		{"LocationReport", tableIDs(locationReportIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDEUTRANCGI, IDTAI, IDRequestType}},
		{"NASNonDeliveryIndication", tableIDs(nASNonDeliveryIndicationIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDNASPDU, IDCause}},
		{"InitialUEMessage", tableIDs(initialUEMessageIEs), []ProtocolIEID{IDENBUES1APID, IDNASPDU, IDTAI, IDEUTRANCGI, IDRRCEstablishmentCause, IDSTMSI, IDGUMMEI}},
		{"UplinkNASTransport", tableIDs(uplinkNASTransportIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDNASPDU, IDEUTRANCGI, IDTAI}},
		{"DownlinkNASTransport", tableIDs(downlinkNASTransportIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDNASPDU}},
		{"Paging", tableIDs(pagingIEs), []ProtocolIEID{IDUEIdentityIndexValue, IDUEPagingID, IDPagingDRX, IDCNDomain, IDTAIList, IDPagingPriority, IDUERadioCapabilityForPaging}},
		{"PathSwitchRequest", tableIDs(pathSwitchRequestIEs), []ProtocolIEID{IDENBUES1APID, IDERABToBeSwitchedDLList, IDSourceMMEUES1APID, IDEUTRANCGI, IDTAI, IDUESecurityCapabilities}},
		{"PathSwitchRequestAcknowledge", tableIDs(pathSwitchRequestAcknowledgeIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDUEAggregateMaximumBitrate, IDERABToBeReleasedList, IDSecurityContext, IDUESecurityCapabilities}},
		{"PathSwitchRequestFailure", tableIDs(pathSwitchRequestFailureIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDCause, IDCriticalityDiagnostics}},
		{"Reset", tableIDs(resetIEs), []ProtocolIEID{IDCause, IDResetType}},
		{"ResetAcknowledge", tableIDs(resetAcknowledgeIEs), []ProtocolIEID{IDUEAssociatedLogicalS1ConnectionListResAck, IDCriticalityDiagnostics}},
		{"S1SetupRequest", tableIDs(s1SetupRequestIEs), []ProtocolIEID{IDGlobalENBID, IDENBname, IDSupportedTAs, IDDefaultPagingDRX, IDUERetentionInformation}},
		{"S1SetupFailure", tableIDs(s1SetupFailureIEs), []ProtocolIEID{IDCause, IDTimeToWait, IDCriticalityDiagnostics}},
		{"S1SetupResponse", tableIDs(s1SetupResponseIEs), []ProtocolIEID{IDMMEname, IDServedGUMMEIs, IDRelativeMMECapacity, IDCriticalityDiagnostics, IDUERetentionInformation}},
		{"ENBStatusTransfer", tableIDs(eNBStatusTransferIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDENBStatusTransferTransparentContainer}},
		{"MMEStatusTransfer", tableIDs(mMEStatusTransferIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDENBStatusTransferTransparentContainer}},
		{"UECapabilityInfoIndication", tableIDs(uECapabilityInfoIndicationIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDUERadioCapability, IDUERadioCapabilityForPaging}},
		{"UEContextReleaseCommand", tableIDs(uEContextReleaseCommandIEs), []ProtocolIEID{IDUES1APIDs, IDCause}},
		{"UEContextReleaseComplete", tableIDs(uEContextReleaseCompleteIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDCriticalityDiagnostics, IDUserLocationInformation}},
		{"UEContextReleaseRequest", tableIDs(uEContextReleaseRequestIEs), []ProtocolIEID{IDMMEUES1APID, IDENBUES1APID, IDCause}},
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
