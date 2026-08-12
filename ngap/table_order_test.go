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
		{"AMFStatusIndication", tableIDs(aMFStatusIndicationIEs), []ProtocolIEID{IDUnavailableGUAMIList}},
		{"ErrorIndication", tableIDs(errorIndicationIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDCause, IDCriticalityDiagnostics, IDFiveGSTMSI}},
		{"NGReset", tableIDs(nGResetIEs), []ProtocolIEID{IDCause, IDResetType}},
		{"NGResetAcknowledge", tableIDs(nGResetAcknowledgeIEs), []ProtocolIEID{IDUEAssociatedLogicalNGConnectionList, IDCriticalityDiagnostics}},
		{"NGSetupRequest", tableIDs(nGSetupRequestIEs), []ProtocolIEID{IDGlobalRANNodeID, IDRANNodeName, IDSupportedTAList, IDDefaultPagingDRX, IDUERetentionInformation}},
		{"NGSetupResponse", tableIDs(nGSetupResponseIEs), []ProtocolIEID{IDAMFName, IDServedGUAMIList, IDRelativeAMFCapacity, IDPLMNSupportList, IDCriticalityDiagnostics, IDUERetentionInformation}},
		{"NGSetupFailure", tableIDs(nGSetupFailureIEs), []ProtocolIEID{IDCause, IDTimeToWait, IDCriticalityDiagnostics}},
		{"NASNonDeliveryIndication", tableIDs(nASNonDeliveryIndicationIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDNASPDU, IDCause}},
		{"InitialUEMessage", tableIDs(initialUEMessageIEs), []ProtocolIEID{IDRANUENGAPID, IDNASPDU, IDUserLocationInformation, IDRRCEstablishmentCause, IDFiveGSTMSI, IDAMFSetID, IDUEContextRequest, IDAllowedNSSAI}},
		{"DownlinkNASTransport", tableIDs(downlinkNASTransportIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDNASPDU}},
		{"UplinkNASTransport", tableIDs(uplinkNASTransportIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDNASPDU, IDUserLocationInformation}},
		{"InitialContextSetupRequest", tableIDs(initialContextSetupRequestIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDUEAggregateMaximumBitRate, IDGUAMI, IDPDUSessionResourceSetupListCxtReq, IDAllowedNSSAI, IDUESecurityCapabilities, IDSecurityKey, IDUERadioCapability, IDNASPDU, IDUERadioCapabilityForPaging}},
		{"InitialContextSetupResponse", tableIDs(initialContextSetupResponseIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceSetupListCxtRes, IDPDUSessionResourceFailedToSetupListCxtRes, IDCriticalityDiagnostics}},
		{"InitialContextSetupFailure", tableIDs(initialContextSetupFailureIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceFailedToSetupListCxtFail, IDCause, IDCriticalityDiagnostics}},
		{"HandoverRequired", tableIDs(handoverRequiredIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDHandoverType, IDCause, IDTargetID, IDDirectForwardingPathAvailability, IDPDUSessionResourceListHORqd, IDSourceToTargetTransparentContainer}},
		{"HandoverCommand", tableIDs(handoverCommandIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDHandoverType, IDNASSecurityParametersFromNGRAN, IDPDUSessionResourceHandoverList, IDPDUSessionResourceToReleaseListHOCmd, IDTargetToSourceTransparentContainer, IDCriticalityDiagnostics}},
		{"HandoverPreparationFailure", tableIDs(handoverPreparationFailureIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDCause, IDCriticalityDiagnostics, IDTargettoSourceFailureTransparentContainer}},
		{"HandoverRequest", tableIDs(handoverRequestIEs), []ProtocolIEID{IDAMFUENGAPID, IDHandoverType, IDCause, IDUEAggregateMaximumBitRate, IDUESecurityCapabilities, IDSecurityContext, IDNewSecurityContextInd, IDNASC, IDPDUSessionResourceSetupListHOReq, IDAllowedNSSAI, IDSourceToTargetTransparentContainer, IDMobilityRestrictionList, IDGUAMI}},
		{"HandoverRequestAcknowledge", tableIDs(handoverRequestAcknowledgeIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceAdmittedList, IDPDUSessionResourceFailedToSetupListHOAck, IDTargetToSourceTransparentContainer, IDCriticalityDiagnostics}},
		{"HandoverFailure", tableIDs(handoverFailureIEs), []ProtocolIEID{IDAMFUENGAPID, IDCause, IDCriticalityDiagnostics, IDTargettoSourceFailureTransparentContainer}},
		{"DownlinkUEAssociatedNRPPaTransport", tableIDs(downlinkUEAssociatedNRPPaTransportIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDRoutingID, IDNRPPaPDU}},
		{"UplinkUEAssociatedNRPPaTransport", tableIDs(uplinkUEAssociatedNRPPaTransportIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDRoutingID, IDNRPPaPDU}},
		{"DownlinkNonUEAssociatedNRPPaTransport", tableIDs(downlinkNonUEAssociatedNRPPaTransportIEs), []ProtocolIEID{IDRoutingID, IDNRPPaPDU}},
		{"UplinkNonUEAssociatedNRPPaTransport", tableIDs(uplinkNonUEAssociatedNRPPaTransportIEs), []ProtocolIEID{IDRoutingID, IDNRPPaPDU}},
		{"LocationReport", tableIDs(locationReportIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDUserLocationInformation, IDUEPresenceInAreaOfInterestList, IDLocationReportingRequestType}},
		{"LocationReportingControl", tableIDs(locationReportingControlIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDLocationReportingRequestType}},
		{"PathSwitchRequest", tableIDs(pathSwitchRequestIEs), []ProtocolIEID{IDRANUENGAPID, IDSourceAMFUENGAPID, IDUserLocationInformation, IDUESecurityCapabilities, IDPDUSessionResourceToBeSwitchedDLList, IDPDUSessionResourceFailedToSetupListPSReq}},
		{"PathSwitchRequestAcknowledge", tableIDs(pathSwitchRequestAcknowledgeIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDUESecurityCapabilities, IDSecurityContext, IDPDUSessionResourceSwitchedList, IDPDUSessionResourceReleasedListPSAck, IDAllowedNSSAI, IDCriticalityDiagnostics}},
		{"PathSwitchRequestFailure", tableIDs(pathSwitchRequestFailureIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceReleasedListPSFail, IDCriticalityDiagnostics}},
		{"UplinkRANStatusTransfer", tableIDs(uplinkRANStatusTransferIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDRANStatusTransferTransparentContainer}},
		{"DownlinkRANStatusTransfer", tableIDs(downlinkRANStatusTransferIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDRANStatusTransferTransparentContainer}},
		{"HandoverNotify", tableIDs(handoverNotifyIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDUserLocationInformation, IDNotifySourceNGRANNode}},
		{"HandoverCancel", tableIDs(handoverCancelIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDCause}},
		{"HandoverCancelAcknowledge", tableIDs(handoverCancelAcknowledgeIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDCriticalityDiagnostics}},
		{"PDUSessionResourceNotify", tableIDs(pDUSessionResourceNotifyIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceNotifyList, IDPDUSessionResourceReleasedListNot, IDUserLocationInformation}},
		{"PDUSessionResourceModifyIndication", tableIDs(pDUSessionResourceModifyIndicationIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceModifyListModInd, IDUserLocationInformation}},
		{"PDUSessionResourceModifyConfirm", tableIDs(pDUSessionResourceModifyConfirmIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceModifyListModCfm, IDPDUSessionResourceFailedToModifyListModCfm, IDCriticalityDiagnostics}},
		{"PDUSessionResourceModifyRequest", tableIDs(pDUSessionResourceModifyRequestIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceModifyListModReq}},
		{"PDUSessionResourceModifyResponse", tableIDs(pDUSessionResourceModifyResponseIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceModifyListModRes, IDPDUSessionResourceFailedToModifyListModRes, IDUserLocationInformation, IDCriticalityDiagnostics}},
		{"PDUSessionResourceReleaseCommand", tableIDs(pDUSessionResourceReleaseCommandIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDNASPDU, IDPDUSessionResourceToReleaseListRelCmd}},
		{"PDUSessionResourceReleaseResponse", tableIDs(pDUSessionResourceReleaseResponseIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceReleasedListRelRes, IDUserLocationInformation, IDCriticalityDiagnostics}},
		{"PDUSessionResourceSetupRequest", tableIDs(pDUSessionResourceSetupRequestIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDNASPDU, IDPDUSessionResourceSetupListSUReq, IDUEAggregateMaximumBitRate}},
		{"PDUSessionResourceSetupResponse", tableIDs(pDUSessionResourceSetupResponseIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceSetupListSURes, IDPDUSessionResourceFailedToSetupListSURes, IDCriticalityDiagnostics, IDUserLocationInformation}},
		{"UERadioCapabilityInfoIndication", tableIDs(uERadioCapabilityInfoIndicationIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDUERadioCapability, IDUERadioCapabilityForPaging}},
		{"UEContextReleaseRequest", tableIDs(uEContextReleaseRequestIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDPDUSessionResourceListCxtRelReq, IDCause}},
		{"UEContextReleaseCommand", tableIDs(uEContextReleaseCommandIEs), []ProtocolIEID{IDUENGAPIDs, IDCause}},
		{"UEContextReleaseComplete", tableIDs(uEContextReleaseCompleteIEs), []ProtocolIEID{IDAMFUENGAPID, IDRANUENGAPID, IDUserLocationInformation, IDPDUSessionResourceListCxtRelCpl, IDCriticalityDiagnostics}},
		{"Paging", tableIDs(pagingIEs), []ProtocolIEID{IDUEPagingIdentity, IDPagingDRX, IDTAIListForPaging, IDPagingPriority, IDUERadioCapabilityForPaging, IDPagingOrigin}},
		{"RANConfigurationUpdate", tableIDs(rANConfigurationUpdateIEs), []ProtocolIEID{IDRANNodeName, IDSupportedTAList, IDDefaultPagingDRX, IDGlobalRANNodeID, IDNGRANTNLAssociationToRemoveList}},
		{"RANConfigurationUpdateAcknowledge", tableIDs(rANConfigurationUpdateAcknowledgeIEs), []ProtocolIEID{IDCriticalityDiagnostics}},
		{"UplinkRANConfigurationTransfer", tableIDs(uplinkRANConfigurationTransferIEs), []ProtocolIEID{IDSONConfigurationTransferUL}},
		{"DownlinkRANConfigurationTransfer", tableIDs(downlinkRANConfigurationTransferIEs), []ProtocolIEID{IDSONConfigurationTransferDL}},
		{"RANConfigurationUpdateFailure", tableIDs(rANConfigurationUpdateFailureIEs), []ProtocolIEID{IDCause, IDTimeToWait, IDCriticalityDiagnostics}},

		// The §9.3.4 transfer containers reuse the ProtocolIE-Container shape, so
		// their rows are ordered by the same rule.
		{"PDUSessionResourceSetupRequestTransfer", tableIDs(pDUSessionResourceSetupRequestTransferIEs), []ProtocolIEID{IDPDUSessionAggregateMaximumBitRate, IDULNGUUPTNLInformation, IDAdditionalULNGUUPTNLInformation, IDDataForwardingNotPossible, IDPDUSessionType, IDSecurityIndication, IDNetworkInstance, IDQosFlowSetupRequestList}},
		{"PDUSessionResourceModifyRequestTransfer", tableIDs(pDUSessionResourceModifyRequestTransferIEs), []ProtocolIEID{IDPDUSessionAggregateMaximumBitRate, IDULNGUUPTNLModifyList, IDNetworkInstance, IDQosFlowAddOrModifyRequestList, IDQosFlowToReleaseList, IDAdditionalULNGUUPTNLInformation}},
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
