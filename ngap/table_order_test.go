// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"slices"
	"testing"
)

// TestTableOrderMatchesASN1 pins the IEs each table models to their relative
// order in the message's NGAP-PROTOCOL-IES container (TS 38.413 §9.4.5), which
// is the order encode emits them in. IEs a table does not model are not pinned.
func TestTableOrderMatchesASN1(t *testing.T) {
	tests := []struct {
		name      string
		got       []tableRowSpec
		asn1      string
		container string
	}{
		{"AMFStatusIndication", tableRows(aMFStatusIndicationIEs), "AMFStatusIndication", "ProtocolIE-Container"},
		{"ErrorIndication", tableRows(errorIndicationIEs), "ErrorIndication", "ProtocolIE-Container"},
		{"NGReset", tableRows(nGResetIEs), "NGReset", "ProtocolIE-Container"},
		{"NGResetAcknowledge", tableRows(nGResetAcknowledgeIEs), "NGResetAcknowledge", "ProtocolIE-Container"},
		{"NGSetupRequest", tableRows(nGSetupRequestIEs), "NGSetupRequest", "ProtocolIE-Container"},
		{"NGSetupResponse", tableRows(nGSetupResponseIEs), "NGSetupResponse", "ProtocolIE-Container"},
		{"NGSetupFailure", tableRows(nGSetupFailureIEs), "NGSetupFailure", "ProtocolIE-Container"},
		{"NASNonDeliveryIndication", tableRows(nASNonDeliveryIndicationIEs), "NASNonDeliveryIndication", "ProtocolIE-Container"},
		{"InitialUEMessage", tableRows(initialUEMessageIEs), "InitialUEMessage", "ProtocolIE-Container"},
		{"DownlinkNASTransport", tableRows(downlinkNASTransportIEs), "DownlinkNASTransport", "ProtocolIE-Container"},
		{"UplinkNASTransport", tableRows(uplinkNASTransportIEs), "UplinkNASTransport", "ProtocolIE-Container"},
		{"InitialContextSetupRequest", tableRows(initialContextSetupRequestIEs), "InitialContextSetupRequest", "ProtocolIE-Container"},
		{"InitialContextSetupResponse", tableRows(initialContextSetupResponseIEs), "InitialContextSetupResponse", "ProtocolIE-Container"},
		{"InitialContextSetupFailure", tableRows(initialContextSetupFailureIEs), "InitialContextSetupFailure", "ProtocolIE-Container"},
		{"HandoverRequired", tableRows(handoverRequiredIEs), "HandoverRequired", "ProtocolIE-Container"},
		{"HandoverCommand", tableRows(handoverCommandIEs), "HandoverCommand", "ProtocolIE-Container"},
		{"HandoverPreparationFailure", tableRows(handoverPreparationFailureIEs), "HandoverPreparationFailure", "ProtocolIE-Container"},
		{"HandoverRequest", tableRows(handoverRequestIEs), "HandoverRequest", "ProtocolIE-Container"},
		{"HandoverRequestAcknowledge", tableRows(handoverRequestAcknowledgeIEs), "HandoverRequestAcknowledge", "ProtocolIE-Container"},
		{"HandoverFailure", tableRows(handoverFailureIEs), "HandoverFailure", "ProtocolIE-Container"},
		{"DownlinkUEAssociatedNRPPaTransport", tableRows(downlinkUEAssociatedNRPPaTransportIEs), "DownlinkUEAssociatedNRPPaTransport", "ProtocolIE-Container"},
		{"UplinkUEAssociatedNRPPaTransport", tableRows(uplinkUEAssociatedNRPPaTransportIEs), "UplinkUEAssociatedNRPPaTransport", "ProtocolIE-Container"},
		{"DownlinkNonUEAssociatedNRPPaTransport", tableRows(downlinkNonUEAssociatedNRPPaTransportIEs), "DownlinkNonUEAssociatedNRPPaTransport", "ProtocolIE-Container"},
		{"UplinkNonUEAssociatedNRPPaTransport", tableRows(uplinkNonUEAssociatedNRPPaTransportIEs), "UplinkNonUEAssociatedNRPPaTransport", "ProtocolIE-Container"},
		{"LocationReport", tableRows(locationReportIEs), "LocationReport", "ProtocolIE-Container"},
		{"LocationReportingControl", tableRows(locationReportingControlIEs), "LocationReportingControl", "ProtocolIE-Container"},
		{"PathSwitchRequest", tableRows(pathSwitchRequestIEs), "PathSwitchRequest", "ProtocolIE-Container"},
		{"PathSwitchRequestAcknowledge", tableRows(pathSwitchRequestAcknowledgeIEs), "PathSwitchRequestAcknowledge", "ProtocolIE-Container"},
		{"PathSwitchRequestFailure", tableRows(pathSwitchRequestFailureIEs), "PathSwitchRequestFailure", "ProtocolIE-Container"},
		{"UplinkRANStatusTransfer", tableRows(uplinkRANStatusTransferIEs), "UplinkRANStatusTransfer", "ProtocolIE-Container"},
		{"DownlinkRANStatusTransfer", tableRows(downlinkRANStatusTransferIEs), "DownlinkRANStatusTransfer", "ProtocolIE-Container"},
		{"HandoverNotify", tableRows(handoverNotifyIEs), "HandoverNotify", "ProtocolIE-Container"},
		{"TraceStart", tableRows(traceStartIEs), "TraceStart", "ProtocolIE-Container"},
		{"TraceFailureIndication", tableRows(traceFailureIndicationIEs), "TraceFailureIndication", "ProtocolIE-Container"},
		{"HandoverCancel", tableRows(handoverCancelIEs), "HandoverCancel", "ProtocolIE-Container"},
		{"HandoverCancelAcknowledge", tableRows(handoverCancelAcknowledgeIEs), "HandoverCancelAcknowledge", "ProtocolIE-Container"},
		{"PDUSessionResourceNotify", tableRows(pDUSessionResourceNotifyIEs), "PDUSessionResourceNotify", "ProtocolIE-Container"},
		{"PDUSessionResourceModifyIndication", tableRows(pDUSessionResourceModifyIndicationIEs), "PDUSessionResourceModifyIndication", "ProtocolIE-Container"},
		{"PDUSessionResourceModifyConfirm", tableRows(pDUSessionResourceModifyConfirmIEs), "PDUSessionResourceModifyConfirm", "ProtocolIE-Container"},
		{"PDUSessionResourceModifyRequest", tableRows(pDUSessionResourceModifyRequestIEs), "PDUSessionResourceModifyRequest", "ProtocolIE-Container"},
		{"PDUSessionResourceModifyResponse", tableRows(pDUSessionResourceModifyResponseIEs), "PDUSessionResourceModifyResponse", "ProtocolIE-Container"},
		{"PDUSessionResourceReleaseCommand", tableRows(pDUSessionResourceReleaseCommandIEs), "PDUSessionResourceReleaseCommand", "ProtocolIE-Container"},
		{"PDUSessionResourceReleaseResponse", tableRows(pDUSessionResourceReleaseResponseIEs), "PDUSessionResourceReleaseResponse", "ProtocolIE-Container"},
		{"PDUSessionResourceSetupRequest", tableRows(pDUSessionResourceSetupRequestIEs), "PDUSessionResourceSetupRequest", "ProtocolIE-Container"},
		{"PDUSessionResourceSetupResponse", tableRows(pDUSessionResourceSetupResponseIEs), "PDUSessionResourceSetupResponse", "ProtocolIE-Container"},
		{"UERadioCapabilityInfoIndication", tableRows(uERadioCapabilityInfoIndicationIEs), "UERadioCapabilityInfoIndication", "ProtocolIE-Container"},
		{"UEContextModificationRequest", tableRows(uEContextModificationRequestIEs), "UEContextModificationRequest", "ProtocolIE-Container"},
		{"UEContextModificationResponse", tableRows(uEContextModificationResponseIEs), "UEContextModificationResponse", "ProtocolIE-Container"},
		{"UEContextModificationFailure", tableRows(uEContextModificationFailureIEs), "UEContextModificationFailure", "ProtocolIE-Container"},
		{"UEContextReleaseRequest", tableRows(uEContextReleaseRequestIEs), "UEContextReleaseRequest", "ProtocolIE-Container"},
		{"UEContextReleaseCommand", tableRows(uEContextReleaseCommandIEs), "UEContextReleaseCommand", "ProtocolIE-Container"},
		{"UEContextReleaseComplete", tableRows(uEContextReleaseCompleteIEs), "UEContextReleaseComplete", "ProtocolIE-Container"},
		{"Paging", tableRows(pagingIEs), "Paging", "ProtocolIE-Container"},
		{"RANConfigurationUpdate", tableRows(rANConfigurationUpdateIEs), "RANConfigurationUpdate", "ProtocolIE-Container"},
		{"AMFConfigurationUpdate", tableRows(aMFConfigurationUpdateIEs), "AMFConfigurationUpdate", "ProtocolIE-Container"},
		{"PLMNSupportItem extensions", tableRows(extIEsPLMNSupportItem), "PLMNSupportItem", "ProtocolExtensionContainer"},
		{"AMFTNLAssociationToRemoveItem extensions", tableRows(extIEsAMFTNLAssociationToRemoveItem), "AMF-TNLAssociationToRemoveItem", "ProtocolExtensionContainer"},
		{"ServedGUAMIItem extensions", tableRows(extIEsServedGUAMIItem), "ServedGUAMIItem", "ProtocolExtensionContainer"},
		{"UnavailableGUAMIItem extensions", tableRows(extIEsUnavailableGUAMIItem), "UnavailableGUAMIItem", "ProtocolExtensionContainer"},
		{"OverloadStart", tableRows(overloadStartIEs), "OverloadStart", "ProtocolIE-Container"},
		{"OverloadStop", tableRows(overloadStopIEs), "OverloadStop", "ProtocolIE-Container"},
		{"AMFConfigurationUpdateAcknowledge", tableRows(aMFConfigurationUpdateAcknowledgeIEs), "AMFConfigurationUpdateAcknowledge", "ProtocolIE-Container"},
		{"UETNLABindingReleaseRequest", tableRows(uETNLABindingReleaseRequestIEs), "UETNLABindingReleaseRequest", "ProtocolIE-Container"},
		{"AMFConfigurationUpdateFailure", tableRows(aMFConfigurationUpdateFailureIEs), "AMFConfigurationUpdateFailure", "ProtocolIE-Container"},
		{"RANConfigurationUpdateAcknowledge", tableRows(rANConfigurationUpdateAcknowledgeIEs), "RANConfigurationUpdateAcknowledge", "ProtocolIE-Container"},
		{"UplinkRANConfigurationTransfer", tableRows(uplinkRANConfigurationTransferIEs), "UplinkRANConfigurationTransfer", "ProtocolIE-Container"},
		{"DownlinkRANConfigurationTransfer", tableRows(downlinkRANConfigurationTransferIEs), "DownlinkRANConfigurationTransfer", "ProtocolIE-Container"},
		{"RANConfigurationUpdateFailure", tableRows(rANConfigurationUpdateFailureIEs), "RANConfigurationUpdateFailure", "ProtocolIE-Container"},

		// The §9.3.4 transfer containers reuse the ProtocolIE-Container shape, so
		// their rows are ordered by the same rule.
		{"PDUSessionResourceSetupRequestTransfer", tableRows(pDUSessionResourceSetupRequestTransferIEs), "PDUSessionResourceSetupRequestTransfer", "ProtocolIE-Container"},
		{"PDUSessionResourceModifyRequestTransfer", tableRows(pDUSessionResourceModifyRequestTransferIEs), "PDUSessionResourceModifyRequestTransfer", "ProtocolIE-Container"},
		{"PDUSessionResourceSetupResponseTransfer extensions", tableRows(extIEsPDUSessionResourceSetupResponseTransfer), "PDUSessionResourceSetupResponseTransfer", "ProtocolExtensionContainer"},
		{"PDUSessionResourceModifyResponseTransfer extensions", tableRows(extIEsPDUSessionResourceModifyResponseTransfer), "PDUSessionResourceModifyResponseTransfer", "ProtocolExtensionContainer"},
		{"PDUSessionResourceModifyIndicationTransfer extensions", tableRows(extIEsPDUSessionResourceModifyIndicationTransfer), "PDUSessionResourceModifyIndicationTransfer", "ProtocolExtensionContainer"},
		{"PDUSessionResourceModifyConfirmTransfer extensions", tableRows(extIEsPDUSessionResourceModifyConfirmTransfer), "PDUSessionResourceModifyConfirmTransfer", "ProtocolExtensionContainer"},
		{"PathSwitchRequestTransfer extensions", tableRows(extIEsPathSwitchRequestTransfer), "PathSwitchRequestTransfer", "ProtocolExtensionContainer"},
		{"PathSwitchRequestAcknowledgeTransfer extensions", tableRows(extIEsPathSwitchRequestAcknowledgeTransfer), "PathSwitchRequestAcknowledgeTransfer", "ProtocolExtensionContainer"},
		{"HandoverRequestAcknowledgeTransfer extensions", tableRows(extIEsHandoverRequestAcknowledgeTransfer), "HandoverRequestAcknowledgeTransfer", "ProtocolExtensionContainer"},
		{"HandoverCommandTransfer extensions", tableRows(extIEsHandoverCommandTransfer), "HandoverCommandTransfer", "ProtocolExtensionContainer"},
		{"UL-NGU-UP-TNLModifyItem extensions", tableRows(extIEsULNGUUPTNLModifyItem), "UL-NGU-UP-TNLModifyItem", "ProtocolExtensionContainer"},
		{"AdditionalDLUPTNLInformationForHOItem extensions", tableRows(extIEsAdditionalDLUPTNLInformationForHOItem), "AdditionalDLUPTNLInformationForHOItem", "ProtocolExtensionContainer"},
		{"QosFlowAddOrModifyRequestItem extensions", tableRows(extIEsQosFlowAddOrModifyRequestItem), "QosFlowAddOrModifyRequestItem", "ProtocolExtensionContainer"},
		{"QosFlowAddOrModifyResponseItem extensions", tableRows(extIEsQosFlowAddOrModifyResponseItem), "QosFlowAddOrModifyResponseItem", "ProtocolExtensionContainer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := asn1Rows(t, tt.asn1, tt.container)
			if !slices.Equal(tt.got, want) {
				t.Fatalf("rows\n  %v\nwant\n  %v", tt.got, want)
			}
		})
	}
}
