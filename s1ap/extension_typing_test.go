// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/per/ap"
)

type extSpecRow struct {
	id       ProtocolIEID
	crit     string
	presence string
}

func extSpecRows[M any](table []ieSpec[M]) []extSpecRow {
	rows := make([]extSpecRow, len(table))

	for i, s := range table {
		p := "optional"

		switch s.Presence {
		case ap.PresenceMandatory:
			p = "mandatory"
		case ap.PresenceConditional:
			p = "conditional"
		}

		rows[i] = extSpecRow{id: s.ID, crit: s.Criticality.String(), presence: p}
	}

	return rows
}

type extTableCase struct {
	set  string
	rows []extSpecRow
	typ  reflect.Type
}

func extTable[M any](set string, table []ieSpec[M]) extTableCase {
	return extTableCase{set: set, rows: extSpecRows(table), typ: reflect.TypeFor[M]()}
}

func typedExtensionTables() []extTableCase {
	return []extTableCase{
		extTable("Additional-GUTI-ExtIEs", extIEsAdditionalGUTI),
		extTable("AllocationAndRetentionPriority-ExtIEs", extIEsAllocationAndRetentionPriority),
		extTable("CGI-ExtIEs", extIEsCGI),
		extTable("CNTypeRestrictions-Item-ExtIEs", extIEsCNTypeRestrictionsItem),
		extTable("CancelledCellinEAI-Item-ExtIEs", extIEsCancelledCellInEAIItem),
		extTable("CancelledCellinTAI-Item-ExtIEs", extIEsCancelledCellInTAIItem),
		extTable("CellID-Broadcast-Item-ExtIEs", extIEsCellIDBroadcastItem),
		extTable("CellID-Cancelled-Item-ExtIEs", extIEsCellIDCancelledItem),
		extTable("CellType-ExtIEs", extIEsCellType),
		extTable("CompletedCellinEAI-Item-ExtIEs", extIEsCompletedCellInEAIItem),
		extTable("CompletedCellinTAI-Item-ExtIEs", extIEsCompletedCellInTAIItem),
		extTable("ContextatSource-ExtIEs", extIEsContextatSource),
		extTable("CriticalityDiagnostics-ExtIEs", extIEsCriticalityDiagnostics),
		extTable("CriticalityDiagnostics-IE-Item-ExtIEs", extIEsCriticalityDiagnosticsIEItem),
		extTable("DAPSRequestInfo-ExtIEs", extIEsDAPSRequestInfo),
		extTable("ENBX2ExtTLA-ExtIEs", extIEsENBX2ExtTLA),
		extTable("E-RABAdmittedItem-ExtIEs", extIEsERABAdmittedItem),
		extTable("E-RABDataForwardingItem-ExtIEs", extIEsERABDataForwardingItem),
		extTable("E-RABInformationListItem-ExtIEs", extIEsERABInformationListItem),
		extTable("E-RABItem-ExtIEs", extIEsERABItem),
		extTable("E-RABQoSParameters-ExtIEs", extIEsERABLevelQoSParameters),
		extTable("E-RABModifyItemBearerModResExtIEs", extIEsERABModifyItemBearerModRes),
		extTable("E-RABReleaseItemBearerRelCompExtIEs", extIEsERABReleaseItemBearerRelComp),
		extTable("E-RABSetupItemCtxtSUResExtIEs", extIEsERABSetupItemCtxtSURes),
		extTable("E-RABToBeModifiedItemBearerModInd-ExtIEs", extIEsERABToBeModifiedItemBearerModInd),
		extTable("E-RABToBeModifyItemBearerModReqExtIEs", extIEsERABToBeModifiedItemBearerModReq),
		extTable("E-RABToBeSetupItemBearerSUReqExtIEs", extIEsERABToBeSetupItemBearerSUReq),
		extTable("E-RABToBeSetupItemCtxtSUReqExtIEs", extIEsERABToBeSetupItemCtxtSUReq),
		extTable("E-RABToBeSetupItemHOReq-ExtIEs", extIEsERABToBeSetupItemHOReq),
		extTable("E-RABToBeSwitchedDLItem-ExtIEs", extIEsERABToBeSwitchedDLItem),
		extTable("E-RABToBeSwitchedULItem-ExtIEs", extIEsERABToBeSwitchedULItem),
		extTable("EUTRAN-CGI-ExtIEs", extIEsEUTRANCGI),
		extTable("EmergencyAreaID-Broadcast-Item-ExtIEs", extIEsEmergencyAreaIDBroadcastItem),
		extTable("EmergencyAreaID-Cancelled-Item-ExtIEs", extIEsEmergencyAreaIDCancelledItem),
		extTable("FiveGSTAI-ExtIEs", extIEsFiveGSTAI),
		extTable("ForbiddenLAs-Item-ExtIEs", extIEsForbiddenLAsItem),
		extTable("ForbiddenTAs-Item-ExtIEs", extIEsForbiddenTAsItem),
		extTable("GBR-QosInformation-ExtIEs", extIEsGBRQosInformation),
		extTable("GNB-ExtIEs", extIEsGNB),
		extTable("GUMMEI-ExtIEs", extIEsGUMMEI),
		extTable("GlobalENB-ID-ExtIEs", extIEsGlobalENBID),
		extTable("Global-GNB-ID-ExtIEs", extIEsGlobalGNBID),
		extTable("HandoverRestrictionList-ExtIEs", extIEsHandoverRestrictionList),
		extTable("InterSystemMeasurementItem-ExtIEs", extIEsInterSystemMeasurementItem),
		extTable("InterSystemMeasurementParameters-ExtIEs", extIEsInterSystemMeasurementParameters),
		extTable("IntersystemMeasurementConfiguration-ExtIEs", extIEsIntersystemMeasurementConfiguration),
		extTable("LAI-ExtIEs", extIEsLAI),
		extTable("LTE-NTN-TAI-Information-ExtIEs", extIEsLTENTNTAIInformation),
		extTable("LastVisitedEUTRANCellInformation-ExtIEs", extIEsLastVisitedEUTRANCellInformation),
		extTable("LastVisitedPSCellInformation-ExtIEs", extIEsLastVisitedPSCellInformation),
		extTable("ListeningSubframePattern-ExtIEs", extIEsListeningSubframePattern),
		extTable("MutingPatternInformation-ExtIEs", extIEsMutingPatternInformation),
		extTable("NR-CGI-ExtIEs", extIEsNRCGI),
		extTable("ProSeAuthorized-ExtIEs", extIEsProSeAuthorized),
		extTable("NG-eNB-ExtIEs", extIEsNgENB),
		extTable("PSCellInformation-ExtIEs", extIEsPSCellInformation),
		extTable("RAT-RestrictionsItem-ExtIEs", extIEsRATRestrictionsItem),
		extTable("RLFReportInformation-ExtIEs", extIEsRLFReportInformation),
		extTable("RequestType-ExtIEs", extIEsRequestType),
		extTable("RequestedTNLInfo-ExtIEs", extIEsRequestedTNLInfo),
		extTable("SONConfigurationTransfer-ExtIEs", extIEsSONConfigurationTransfer),
		extTable("SONInformationReply-ExtIEs", extIEsSONInformationReply),
		extTable("S-TMSI-ExtIEs", extIEsSTMSI),
		extTable("SecurityContext-ExtIEs", extIEsSecurityContext),
		extTable("SecurityIndication-ExtIEs", extIEsSecurityIndication),
		extTable("ServedGUMMEIsItem-ExtIEs", extIEsServedGUMMEIsItem),
		extTable("SourceNgRanNode-ID-ExtIEs", extIEsSourceNgRanNodeID),
		extTable("SourceeNB-ID-ExtIEs", extIEsSourceeNBID),
		extTable("SourceeNB-ToTargeteNB-TransparentContainer-ExtIEs", extIEsSourceeNBToTargeteNBTransparentContainer),
		extTable("SupportedTAs-Item-ExtIEs", extIEsSupportedTAItem),
		extTable("SynchronisationInformation-ExtIEs", extIEsSynchronisationInformation),
		extTable("TAI-ExtIEs", extIEsTAI),
		extTable("TAI-Broadcast-Item-ExtIEs", extIEsTAIBroadcastItem),
		extTable("TAI-Cancelled-Item-ExtIEs", extIEsTAICancelledItem),
		extTable("TAIItemExtIEs", extIEsTAIItem),
		extTable("TargetNgRanNode-ID-ExtIEs", extIEsTargetNgRanNodeID),
		extTable("TargetRNC-ID-ExtIEs", extIEsTargetRNCID),
		extTable("TargeteNB-ID-ExtIEs", extIEsTargeteNBID),
		extTable("TimeBasedHandoverInformation-ExtIEs", extIEsTimeBasedHandoverInformation),
		extTable("TimeSynchronisationInfo-ExtIEs", extIEsTimeSynchronisationInfo),
		extTable("TraceActivation-ExtIEs", extIEsTraceActivation),
		extTable("Tunnel-Information-ExtIEs", extIEsTunnelInformation),
		extTable("UEAggregate-MaximumBitrates-ExtIEs", extIEsUEAggregateMaximumBitRate),
		extTable("UE-associatedLogicalS1-ConnectionItemExtIEs", extIEsUEAssociatedLogicalS1ConnectionItem),
		extTable("UE-S1AP-ID-pair-ExtIEs", extIEsUES1APIDPair),
		extTable("UESecurityCapabilities-ExtIEs", extIEsUESecurityCapabilities),
		extTable("UserLocationInformation-ExtIEs", extIEsUserLocationInformation),
		extTable("X2TNLConfigurationInfo-ExtIEs", extIEsX2TNLConfigurationInfo),
		extTable("E-RABModifyItemBearerModConfExtIEs", extIEserabModifyItemBearerModConf),
		extTable("InformationForCECapableUEs-ExtIEs", extIEsAssistanceDataForCECapableUEs),
		extTable("AssistanceDataForPaging-ExtIEs", extIEsAssistanceDataForPaging),
		extTable("AssistanceDataForRecommendedCells-ExtIEs", extIEsAssistanceDataForRecommendedCells),
		extTable("CSG-IdList-Item-ExtIEs", extIEsCSGIdListItem),
		extTable("CSGMembershipInfo-ExtIEs", extIEsCSGMembershipInfo),
		extTable("CellIdentifierAndCELevelForCECapableUEs-ExtIEs", extIEsCellIdentifierAndCELevelForCECapableUEs),
		extTable("ConnectedengNBItem-ExtIEs", extIEsConnectedengNBItem),
		extTable("EN-DCSONConfigurationTransfer-ExtIEs", extIEsENDCSONConfigurationTransfer),
		extTable("EN-DCSONeNBIdentification-ExtIEs", extIEsENDCSONeNBIdentification),
		extTable("EN-DCSONengNBIdentification-ExtIEs", extIEsENDCSONengNBIdentification),
		extTable("EN-DCTransferTypeReply-ExtIEs", extIEsENDCTransferTypeReply),
		extTable("EN-DCTransferTypeRequest-ExtIEs", extIEsENDCTransferTypeRequest),
		extTable("E-RABNotToBeModifiedItemBearerModInd-ExtIEs", extIEsERABNotToBeModifiedItemBearerModInd),
		extTable("E-RABSetupItemBearerSUResExtIEs", extIEsERABSetupItemBearerSURes),
		extTable("E-RABToBeUpdatedItem-ExtIEs", extIEsERABToBeUpdatedItem),
		extTable("E-RABUsageReportItem-ExtIEs", extIEsERABUsageReportItem),
		extTable("ExpectedUEActivityBehaviour-ExtIEs", extIEsExpectedUEActivityBehaviour),
		extTable("ExpectedUEBehaviour-ExtIEs", extIEsExpectedUEBehaviour),
		extTable("Global-en-gNB-ID-ExtIEs", extIEsGlobalengNBID),
		extTable("InformationOnRecommendedCellsAndENBsForPaging-ExtIEs", extIEsInformationOnRecommendedCellsAndENBsForPaging),
		extTable("NB-IoT-Paging-eDRXInformation-ExtIEs", extIEsNBIoTPagingeDRXInformation),
		extTable("NRUESecurityCapabilities-ExtIEs", extIEsNRUESecurityCapabilities),
		extTable("NRUESidelinkAggregateMaximumBitrate-ExtIEs", extIEsNRUESidelinkAggregateMaximumBitrate),
		extTable("NRV2XServicesAuthorized-ExtIEs", extIEsNRV2XServicesAuthorized),
		extTable("PC5FlowBitRates-ExtIEs", extIEsPC5FlowBitRates),
		extTable("PC5QoSFlowItem-ExtIEs", extIEsPC5QoSFlowItem),
		extTable("PC5QoSParameters-ExtIEs", extIEsPC5QoSParameters),
		extTable("PagingAttemptInformation-ExtIEs", extIEsPagingAttemptInformation),
		extTable("Paging-eDRXInformation-ExtIEs", extIEsPagingeDRXInformation),
		extTable("RecommendedCellsForPagingItem-ExtIEs", extIEsRecommendedCellItem),
		extTable("RecommendedCellsForPaging-ExtIEs", extIEsRecommendedCellsForPaging),
		extTable("RecommendedENBItem-ExtIEs", extIEsRecommendedENBItem),
		extTable("RecommendedENBsForPaging-ExtIEs", extIEsRecommendedENBsForPaging),
		extTable("ScheduledCommunicationTime-ExtIEs", extIEsScheduledCommunicationTime),
		extTable("SecondaryRATDataUsageReportItem-ExtIEs", extIEsSecondaryRATDataUsageReportItem),
		extTable("ServedDCNsItem-ExtIEs", extIEsServedDCNsItem),
		extTable("Subscription-Based-UE-DifferentiationInfo-ExtIEs", extIEsSubscriptionBasedUEDifferentiationInfo),
		extTable("UE-Sidelink-Aggregate-MaximumBitrates-ExtIEs", extIEsUESidelinkAggregateMaximumBitrate),
		extTable("V2XServicesAuthorized-ExtIEs", extIEsV2XServicesAuthorized),
		extTable("WUS-Assistance-Information-ExtIEs", extIEsWUSAssistanceInformation),
	}
}

var knownUntypedExtensions = map[string]string{
	"TraceActivation-ExtIEs/id-MDTConfiguration":     "MDT-Configuration subtree (M1..M7, MDT-Activation, AreaScopeOfMDT); carries no UE id or user-plane TNL",
	"TraceActivation-ExtIEs/id-UEAppLayerMeasConfig": "UEAppLayerMeasConfig subtree (QMC area scope); carries no UE id or user-plane TNL",
}

type asn1ExtRow struct {
	name     string
	id       ProtocolIEID
	crit     string
	presence string
}

func asn1ExtensionSets(t *testing.T) map[string][]asn1ExtRow {
	t.Helper()

	s := vendoredSpec(t)
	sets := map[string][]asn1ExtRow{}

	for _, m := range s.Modules {
		for _, sa := range m.ObjectSets {
			if sa.Class != "S1AP-PROTOCOL-EXTENSION" {
				continue
			}

			objs, err := s.Objects(sa.Set)
			if err != nil {
				t.Fatal(err)
			}

			rows := []asn1ExtRow{}

			for _, o := range objs {
				idv, _ := o.Value("&id")
				crit, _ := o.Value("&criticality")
				pres, _ := o.Value("&presence")

				rows = append(rows, asn1ExtRow{name: idv.Ref, id: asn1Row(t, o).ID, crit: crit.Ref, presence: pres.Ref})
			}

			sets[sa.Name] = rows
		}
	}

	return sets
}

func TestTypedExtensionsMatchTS36413(t *testing.T) {
	sets := asn1ExtensionSets(t)
	seenUntyped := map[string]bool{}

	for _, tbl := range typedExtensionTables() {
		spec, ok := sets[tbl.set]
		if !ok {
			t.Errorf("%s: no such ExtIEs set in the vendored ASN.1", tbl.set)

			continue
		}

		next := 0

		for _, row := range tbl.rows {
			found := -1

			for i := next; i < len(spec); i++ {
				if spec[i].id == row.id {
					found = i

					break
				}
			}

			if found < 0 {
				t.Errorf("%s: typed extension %s is not in the set, or is out of spec order", tbl.set, row.id)

				continue
			}

			if spec[found].crit != row.crit || spec[found].presence != row.presence {
				t.Errorf("%s/%s: typed as %s/%s, spec says %s/%s", tbl.set, spec[found].name, row.crit, row.presence, spec[found].crit, spec[found].presence)
			}

			next = found + 1
		}

		for _, s := range spec {
			typed := false

			for _, row := range tbl.rows {
				typed = typed || row.id == s.id
			}

			key := tbl.set + "/" + s.name

			if typed {
				continue
			}

			if _, allowed := knownUntypedExtensions[key]; !allowed {
				t.Errorf("%s is spec-defined but left untyped", key)
			}

			seenUntyped[key] = true
		}
	}

	for key := range knownUntypedExtensions {
		if !seenUntyped[key] {
			t.Errorf("allowlisted %s is typed now: remove it from knownUntypedExtensions", key)
		}
	}
}

func TestTypedExtensionTablesAreComplete(t *testing.T) {
	cases := typedExtensionTables()

	types := make([]reflect.Type, len(cases))
	for i, c := range cases {
		types[i] = c.typ
	}

	checkComplete(t, "extension table", scannedDecls(t).extTables, types)
}
