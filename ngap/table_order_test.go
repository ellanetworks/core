// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"
)

// TestTableOrderMatchesASN1 pins the IEs each table models to their relative
// order in the message's NGAP-PROTOCOL-IES container (TS 38.413 §9.4.5), which
// is the order encode emits them in. IEs a table does not model are not pinned.
func TestTableOrderMatchesASN1(t *testing.T) {
	tests := []struct {
		name string
		got  []ProtocolIEID
		want []ProtocolIEID
	}{
		{"AMFStatusIndication", tableIDs(aMFStatusIndicationIEs), []ProtocolIEID{idUnavailableGUAMIList}},
		{"ErrorIndication", tableIDs(errorIndicationIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idCause, idCriticalityDiagnostics, idFiveGSTMSI}},
		{"NGReset", tableIDs(nGResetIEs), []ProtocolIEID{idCause, idResetType}},
		{"NGResetAcknowledge", tableIDs(nGResetAcknowledgeIEs), []ProtocolIEID{idUEAssociatedLogicalNGConnectionList, idCriticalityDiagnostics}},
		{"NGSetupRequest", tableIDs(nGSetupRequestIEs), []ProtocolIEID{idGlobalRANNodeID, idRANNodeName, idSupportedTAList, idDefaultPagingDRX, idUERetentionInformation}},
		{"NGSetupResponse", tableIDs(nGSetupResponseIEs), []ProtocolIEID{idAMFName, idServedGUAMIList, idRelativeAMFCapacity, idPLMNSupportList, idCriticalityDiagnostics, idUERetentionInformation}},
		{"NGSetupFailure", tableIDs(nGSetupFailureIEs), []ProtocolIEID{idCause, idTimeToWait, idCriticalityDiagnostics}},
		{"NASNonDeliveryIndication", tableIDs(nASNonDeliveryIndicationIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idNASPDU, idCause}},
		{"InitialUEMessage", tableIDs(initialUEMessageIEs), []ProtocolIEID{idRANUENGAPID, idNASPDU, idUserLocationInformation, idRRCEstablishmentCause, idFiveGSTMSI, idAMFSetID, idUEContextRequest, idAllowedNSSAI}},
		{"DownlinkNASTransport", tableIDs(downlinkNASTransportIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idNASPDU}},
		{"UplinkNASTransport", tableIDs(uplinkNASTransportIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idNASPDU, idUserLocationInformation}},
		{"InitialContextSetupRequest", tableIDs(initialContextSetupRequestIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idUEAggregateMaximumBitRate, idGUAMI, idPDUSessionResourceSetupListCxtReq, idAllowedNSSAI, idUESecurityCapabilities, idSecurityKey, idUERadioCapability, idNASPDU, idUERadioCapabilityForPaging}},
		{"InitialContextSetupResponse", tableIDs(initialContextSetupResponseIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceSetupListCxtRes, idPDUSessionResourceFailedToSetupListCxtRes, idCriticalityDiagnostics}},
		{"InitialContextSetupFailure", tableIDs(initialContextSetupFailureIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceFailedToSetupListCxtFail, idCause, idCriticalityDiagnostics}},
		{"HandoverRequired", tableIDs(handoverRequiredIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idHandoverType, idCause, idTargetID, idDirectForwardingPathAvailability, idPDUSessionResourceListHORqd, idSourceToTargetTransparentContainer}},
		{"HandoverCommand", tableIDs(handoverCommandIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idHandoverType, idNASSecurityParametersFromNGRAN, idPDUSessionResourceHandoverList, idPDUSessionResourceToReleaseListHOCmd, idTargetToSourceTransparentContainer, idCriticalityDiagnostics}},
		{"HandoverPreparationFailure", tableIDs(handoverPreparationFailureIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idCause, idCriticalityDiagnostics}},
		{"HandoverRequest", tableIDs(handoverRequestIEs), []ProtocolIEID{idAMFUENGAPID, idHandoverType, idCause, idUEAggregateMaximumBitRate, idUESecurityCapabilities, idSecurityContext, idPDUSessionResourceSetupListHOReq, idAllowedNSSAI, idSourceToTargetTransparentContainer, idGUAMI}},
		{"HandoverRequestAcknowledge", tableIDs(handoverRequestAcknowledgeIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceAdmittedList, idPDUSessionResourceFailedToSetupListHOAck, idTargetToSourceTransparentContainer, idCriticalityDiagnostics}},
		{"HandoverFailure", tableIDs(handoverFailureIEs), []ProtocolIEID{idAMFUENGAPID, idCause, idCriticalityDiagnostics, idTargettoSourceFailureTransparentContainer}},
		{"LocationReport", tableIDs(locationReportIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idUserLocationInformation, idUEPresenceInAreaOfInterestList, idLocationReportingRequestType}},
		{"LocationReportingControl", tableIDs(locationReportingControlIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idLocationReportingRequestType}},
		{"PathSwitchRequest", tableIDs(pathSwitchRequestIEs), []ProtocolIEID{idRANUENGAPID, idSourceAMFUENGAPID, idUserLocationInformation, idUESecurityCapabilities, idPDUSessionResourceToBeSwitchedDLList, idPDUSessionResourceFailedToSetupListPSReq}},
		{"PathSwitchRequestAcknowledge", tableIDs(pathSwitchRequestAcknowledgeIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idUESecurityCapabilities, idSecurityContext, idPDUSessionResourceSwitchedList, idPDUSessionResourceReleasedListPSAck, idAllowedNSSAI, idCriticalityDiagnostics}},
		{"PathSwitchRequestFailure", tableIDs(pathSwitchRequestFailureIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceReleasedListPSFail, idCriticalityDiagnostics}},
		{"UplinkRANStatusTransfer", tableIDs(uplinkRANStatusTransferIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idRANStatusTransferTransparentContainer}},
		{"DownlinkRANStatusTransfer", tableIDs(downlinkRANStatusTransferIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idRANStatusTransferTransparentContainer}},
		{"HandoverNotify", tableIDs(handoverNotifyIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idUserLocationInformation, idNotifySourceNGRANNode}},
		{"HandoverCancel", tableIDs(handoverCancelIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idCause}},
		{"HandoverCancelAcknowledge", tableIDs(handoverCancelAcknowledgeIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idCriticalityDiagnostics}},
		{"PDUSessionResourceNotify", tableIDs(pDUSessionResourceNotifyIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceNotifyList, idPDUSessionResourceReleasedListNot, idUserLocationInformation}},
		{"PDUSessionResourceModifyIndication", tableIDs(pDUSessionResourceModifyIndicationIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceModifyListModInd, idUserLocationInformation}},
		{"PDUSessionResourceModifyConfirm", tableIDs(pDUSessionResourceModifyConfirmIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceModifyListModCfm, idPDUSessionResourceFailedToModifyListModCfm, idCriticalityDiagnostics}},
		{"PDUSessionResourceModifyRequest", tableIDs(pDUSessionResourceModifyRequestIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceModifyListModReq}},
		{"PDUSessionResourceModifyResponse", tableIDs(pDUSessionResourceModifyResponseIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceModifyListModRes, idPDUSessionResourceFailedToModifyListModRes, idUserLocationInformation, idCriticalityDiagnostics}},
		{"PDUSessionResourceReleaseCommand", tableIDs(pDUSessionResourceReleaseCommandIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idNASPDU, idPDUSessionResourceToReleaseListRelCmd}},
		{"PDUSessionResourceReleaseResponse", tableIDs(pDUSessionResourceReleaseResponseIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceReleasedListRelRes, idUserLocationInformation, idCriticalityDiagnostics}},
		{"PDUSessionResourceSetupRequest", tableIDs(pDUSessionResourceSetupRequestIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idNASPDU, idPDUSessionResourceSetupListSUReq, idUEAggregateMaximumBitRate}},
		{"PDUSessionResourceSetupResponse", tableIDs(pDUSessionResourceSetupResponseIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceSetupListSURes, idPDUSessionResourceFailedToSetupListSURes, idCriticalityDiagnostics, idUserLocationInformation}},
		{"UERadioCapabilityInfoIndication", tableIDs(uERadioCapabilityInfoIndicationIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idUERadioCapability, idUERadioCapabilityForPaging}},
		{"UEContextReleaseRequest", tableIDs(uEContextReleaseRequestIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idPDUSessionResourceListCxtRelReq, idCause}},
		{"UEContextReleaseCommand", tableIDs(uEContextReleaseCommandIEs), []ProtocolIEID{idUENGAPIDs, idCause}},
		{"UEContextReleaseComplete", tableIDs(uEContextReleaseCompleteIEs), []ProtocolIEID{idAMFUENGAPID, idRANUENGAPID, idUserLocationInformation, idPDUSessionResourceListCxtRelCpl, idCriticalityDiagnostics}},
		{"Paging", tableIDs(pagingIEs), []ProtocolIEID{idUEPagingIdentity, idPagingDRX, idTAIListForPaging, idPagingPriority, idUERadioCapabilityForPaging, idPagingOrigin}},
		{"RANConfigurationUpdate", tableIDs(rANConfigurationUpdateIEs), []ProtocolIEID{idRANNodeName, idSupportedTAList, idDefaultPagingDRX, idGlobalRANNodeID, idNGRANTNLAssociationToRemoveList}},
		{"RANConfigurationUpdateAcknowledge", tableIDs(rANConfigurationUpdateAcknowledgeIEs), []ProtocolIEID{idCriticalityDiagnostics}},
		{"UplinkRANConfigurationTransfer", tableIDs(uplinkRANConfigurationTransferIEs), []ProtocolIEID{idSONConfigurationTransferUL}},
		{"DownlinkRANConfigurationTransfer", tableIDs(downlinkRANConfigurationTransferIEs), []ProtocolIEID{idSONConfigurationTransferDL}},
		{"RANConfigurationUpdateFailure", tableIDs(rANConfigurationUpdateFailureIEs), []ProtocolIEID{idCause, idTimeToWait, idCriticalityDiagnostics}},
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
	for i := range table {
		ids[i] = table[i].id
	}

	return ids
}
