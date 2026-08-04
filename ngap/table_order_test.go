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
