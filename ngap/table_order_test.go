// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"reflect"
	"slices"
	"testing"
)

type tableCase struct {
	asn1 string
	rows []tableRowSpec
	typ  reflect.Type
}

func ieTable[M any](asn1 string, table []ieSpec[M]) tableCase {
	return tableCase{asn1: asn1, rows: tableRows(table), typ: reflect.TypeFor[M]()}
}

func tableCases() []tableCase {
	return []tableCase{
		ieTable("AMFConfigurationUpdate", aMFConfigurationUpdateIEs),
		ieTable("AMFConfigurationUpdateAcknowledge", aMFConfigurationUpdateAcknowledgeIEs),
		ieTable("AMFConfigurationUpdateFailure", aMFConfigurationUpdateFailureIEs),
		ieTable("AMFStatusIndication", aMFStatusIndicationIEs),
		ieTable("DownlinkNASTransport", downlinkNASTransportIEs),
		ieTable("DownlinkUEAssociatedNRPPaTransport", downlinkUEAssociatedNRPPaTransportIEs),
		ieTable("UplinkUEAssociatedNRPPaTransport", uplinkUEAssociatedNRPPaTransportIEs),
		ieTable("DownlinkNonUEAssociatedNRPPaTransport", downlinkNonUEAssociatedNRPPaTransportIEs),
		ieTable("UplinkNonUEAssociatedNRPPaTransport", uplinkNonUEAssociatedNRPPaTransportIEs),
		ieTable("DownlinkRANConfigurationTransfer", downlinkRANConfigurationTransferIEs),
		ieTable("DownlinkRANStatusTransfer", downlinkRANStatusTransferIEs),
		ieTable("ErrorIndication", errorIndicationIEs),
		ieTable("HandoverCancel", handoverCancelIEs),
		ieTable("HandoverCancelAcknowledge", handoverCancelAcknowledgeIEs),
		ieTable("HandoverCommand", handoverCommandIEs),
		ieTable("HandoverFailure", handoverFailureIEs),
		ieTable("HandoverNotify", handoverNotifyIEs),
		ieTable("HandoverPreparationFailure", handoverPreparationFailureIEs),
		ieTable("HandoverRequest", handoverRequestIEs),
		ieTable("HandoverRequestAcknowledge", handoverRequestAcknowledgeIEs),
		ieTable("HandoverRequired", handoverRequiredIEs),
		ieTable("InitialContextSetupFailure", initialContextSetupFailureIEs),
		ieTable("InitialContextSetupRequest", initialContextSetupRequestIEs),
		ieTable("InitialContextSetupResponse", initialContextSetupResponseIEs),
		ieTable("InitialUEMessage", initialUEMessageIEs),
		ieTable("LocationReport", locationReportIEs),
		ieTable("LocationReportingControl", locationReportingControlIEs),
		ieTable("NASNonDeliveryIndication", nASNonDeliveryIndicationIEs),
		ieTable("NGReset", nGResetIEs),
		ieTable("NGResetAcknowledge", nGResetAcknowledgeIEs),
		ieTable("NGSetupFailure", nGSetupFailureIEs),
		ieTable("NGSetupRequest", nGSetupRequestIEs),
		ieTable("NGSetupResponse", nGSetupResponseIEs),
		ieTable("OverloadStart", overloadStartIEs),
		ieTable("OverloadStop", overloadStopIEs),
		ieTable("PDUSessionResourceModifyConfirm", pDUSessionResourceModifyConfirmIEs),
		ieTable("PDUSessionResourceModifyIndication", pDUSessionResourceModifyIndicationIEs),
		ieTable("PDUSessionResourceModifyRequest", pDUSessionResourceModifyRequestIEs),
		ieTable("PDUSessionResourceModifyResponse", pDUSessionResourceModifyResponseIEs),
		ieTable("PDUSessionResourceNotify", pDUSessionResourceNotifyIEs),
		ieTable("PDUSessionResourceReleaseCommand", pDUSessionResourceReleaseCommandIEs),
		ieTable("PDUSessionResourceReleaseResponse", pDUSessionResourceReleaseResponseIEs),
		ieTable("PDUSessionResourceSetupRequest", pDUSessionResourceSetupRequestIEs),
		ieTable("PDUSessionResourceSetupResponse", pDUSessionResourceSetupResponseIEs),
		ieTable("PWSCancelRequest", pWSCancelRequestIEs),
		ieTable("PWSCancelResponse", pWSCancelResponseIEs),
		ieTable("PWSFailureIndication", pWSFailureIndicationIEs),
		ieTable("PWSRestartIndication", pWSRestartIndicationIEs),
		ieTable("Paging", pagingIEs),
		ieTable("PathSwitchRequest", pathSwitchRequestIEs),
		ieTable("PathSwitchRequestAcknowledge", pathSwitchRequestAcknowledgeIEs),
		ieTable("PathSwitchRequestFailure", pathSwitchRequestFailureIEs),
		ieTable("RANConfigurationUpdate", rANConfigurationUpdateIEs),
		ieTable("RANConfigurationUpdateAcknowledge", rANConfigurationUpdateAcknowledgeIEs),
		ieTable("RANConfigurationUpdateFailure", rANConfigurationUpdateFailureIEs),
		ieTable("RerouteNASRequest", rerouteNASRequestIEs),
		ieTable("TraceFailureIndication", traceFailureIndicationIEs),
		ieTable("TraceStart", traceStartIEs),
		ieTable("UEContextModificationFailure", uEContextModificationFailureIEs),
		ieTable("UEContextModificationRequest", uEContextModificationRequestIEs),
		ieTable("UEContextModificationResponse", uEContextModificationResponseIEs),
		ieTable("UEContextReleaseCommand", uEContextReleaseCommandIEs),
		ieTable("UEContextReleaseComplete", uEContextReleaseCompleteIEs),
		ieTable("UEContextReleaseRequest", uEContextReleaseRequestIEs),
		ieTable("UERadioCapabilityInfoIndication", uERadioCapabilityInfoIndicationIEs),
		ieTable("UETNLABindingReleaseRequest", uETNLABindingReleaseRequestIEs),
		ieTable("UplinkNASTransport", uplinkNASTransportIEs),
		ieTable("UplinkRANConfigurationTransfer", uplinkRANConfigurationTransferIEs),
		ieTable("UplinkRANStatusTransfer", uplinkRANStatusTransferIEs),
		ieTable("WriteReplaceWarningRequest", writeReplaceWarningRequestIEs),
		ieTable("WriteReplaceWarningResponse", writeReplaceWarningResponseIEs),

		// The §9.3.4 transfer containers reuse the ProtocolIE-Container shape, so
		// their rows are ordered by the same rule.
		ieTable("PDUSessionResourceModifyRequestTransfer", pDUSessionResourceModifyRequestTransferIEs),
		ieTable("PDUSessionResourceSetupRequestTransfer", pDUSessionResourceSetupRequestTransferIEs),
	}
}

// TestTableOrderMatchesASN1 pins each table to the ProtocolIE-Container rows of
// its message (TS 38.413 §9.4.5): every IE, its criticality and presence, in
// the order encode emits them in.
func TestTableOrderMatchesASN1(t *testing.T) {
	for _, c := range tableCases() {
		t.Run(c.asn1, func(t *testing.T) {
			want := asn1Rows(t, c.asn1, "ProtocolIE-Container")
			if !slices.Equal(c.rows, want) {
				t.Fatalf("rows\n  %v\nwant\n  %v", c.rows, want)
			}
		})
	}
}

func TestTableCasesAreComplete(t *testing.T) {
	cases := tableCases()

	types := make([]reflect.Type, len(cases))
	for i, c := range cases {
		types[i] = c.typ
	}

	checkComplete(t, "IE table", scannedDecls(t).ieTables, types)
}

func TestExtensionTableCasesAreComplete(t *testing.T) {
	cases := extTableCases()

	types := make([]reflect.Type, len(cases))
	for i, c := range cases {
		types[i] = c.typ
	}

	checkComplete(t, "extension table", scannedDecls(t).extTables, types)
}

func TestMessageCasesAreComplete(t *testing.T) {
	messages := map[string]bool{}

	for _, ep := range elementaryProcedures(t) {
		for _, m := range ep.messages {
			if m != "" {
				messages[m] = true
			}
		}
	}

	declared := map[string]bool{}

	for _, c := range tableCases() {
		if messages[c.asn1] {
			declared[c.typ.Name()] = true
		}
	}

	cases := messageCases()

	types := make([]reflect.Type, len(cases))
	for i, c := range cases {
		types[i] = c.typ
	}

	checkComplete(t, "message", declared, types)
}
