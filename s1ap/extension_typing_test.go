// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

func typedExtensionTables() []struct {
	set  string
	rows []extSpecRow
} {
	return []struct {
		set  string
		rows []extSpecRow
	}{
		{"Additional-GUTI-ExtIEs", extSpecRows(extIEsAdditionalGUTI)},
		{"AllocationAndRetentionPriority-ExtIEs", extSpecRows(extIEsAllocationAndRetentionPriority)},
		{"CGI-ExtIEs", extSpecRows(extIEsCGI)},
		{"CNTypeRestrictions-Item-ExtIEs", extSpecRows(extIEsCNTypeRestrictionsItem)},
		{"CancelledCellinEAI-Item-ExtIEs", extSpecRows(extIEsCancelledCellInEAIItem)},
		{"CancelledCellinTAI-Item-ExtIEs", extSpecRows(extIEsCancelledCellInTAIItem)},
		{"CellID-Broadcast-Item-ExtIEs", extSpecRows(extIEsCellIDBroadcastItem)},
		{"CellID-Cancelled-Item-ExtIEs", extSpecRows(extIEsCellIDCancelledItem)},
		{"CellType-ExtIEs", extSpecRows(extIEsCellType)},
		{"CompletedCellinEAI-Item-ExtIEs", extSpecRows(extIEsCompletedCellInEAIItem)},
		{"CompletedCellinTAI-Item-ExtIEs", extSpecRows(extIEsCompletedCellInTAIItem)},
		{"ContextatSource-ExtIEs", extSpecRows(extIEsContextatSource)},
		{"CriticalityDiagnostics-ExtIEs", extSpecRows(extIEsCriticalityDiagnostics)},
		{"CriticalityDiagnostics-IE-Item-ExtIEs", extSpecRows(extIEsCriticalityDiagnosticsIEItem)},
		{"DAPSRequestInfo-ExtIEs", extSpecRows(extIEsDAPSRequestInfo)},
		{"ENBX2ExtTLA-ExtIEs", extSpecRows(extIEsENBX2ExtTLA)},
		{"E-RABAdmittedItem-ExtIEs", extSpecRows(extIEsERABAdmittedItem)},
		{"E-RABDataForwardingItem-ExtIEs", extSpecRows(extIEsERABDataForwardingItem)},
		{"E-RABInformationListItem-ExtIEs", extSpecRows(extIEsERABInformationListItem)},
		{"E-RABItem-ExtIEs", extSpecRows(extIEsERABItem)},
		{"E-RABQoSParameters-ExtIEs", extSpecRows(extIEsERABLevelQoSParameters)},
		{"E-RABModifyItemBearerModResExtIEs", extSpecRows(extIEsERABModifyItemBearerModRes)},
		{"E-RABReleaseItemBearerRelCompExtIEs", extSpecRows(extIEsERABReleaseItemBearerRelComp)},
		{"E-RABSetupItemCtxtSUResExtIEs", extSpecRows(extIEsERABSetupItemCtxtSURes)},
		{"E-RABToBeModifiedItemBearerModInd-ExtIEs", extSpecRows(extIEsERABToBeModifiedItemBearerModInd)},
		{"E-RABToBeModifyItemBearerModReqExtIEs", extSpecRows(extIEsERABToBeModifiedItemBearerModReq)},
		{"E-RABToBeSetupItemBearerSUReqExtIEs", extSpecRows(extIEsERABToBeSetupItemBearerSUReq)},
		{"E-RABToBeSetupItemCtxtSUReqExtIEs", extSpecRows(extIEsERABToBeSetupItemCtxtSUReq)},
		{"E-RABToBeSetupItemHOReq-ExtIEs", extSpecRows(extIEsERABToBeSetupItemHOReq)},
		{"E-RABToBeSwitchedDLItem-ExtIEs", extSpecRows(extIEsERABToBeSwitchedDLItem)},
		{"E-RABToBeSwitchedULItem-ExtIEs", extSpecRows(extIEsERABToBeSwitchedULItem)},
		{"EUTRAN-CGI-ExtIEs", extSpecRows(extIEsEUTRANCGI)},
		{"EmergencyAreaID-Broadcast-Item-ExtIEs", extSpecRows(extIEsEmergencyAreaIDBroadcastItem)},
		{"EmergencyAreaID-Cancelled-Item-ExtIEs", extSpecRows(extIEsEmergencyAreaIDCancelledItem)},
		{"FiveGSTAI-ExtIEs", extSpecRows(extIEsFiveGSTAI)},
		{"ForbiddenLAs-Item-ExtIEs", extSpecRows(extIEsForbiddenLAsItem)},
		{"ForbiddenTAs-Item-ExtIEs", extSpecRows(extIEsForbiddenTAsItem)},
		{"GBR-QosInformation-ExtIEs", extSpecRows(extIEsGBRQosInformation)},
		{"GNB-ExtIEs", extSpecRows(extIEsGNB)},
		{"GUMMEI-ExtIEs", extSpecRows(extIEsGUMMEI)},
		{"GlobalENB-ID-ExtIEs", extSpecRows(extIEsGlobalENBID)},
		{"Global-GNB-ID-ExtIEs", extSpecRows(extIEsGlobalGNBID)},
		{"HandoverRestrictionList-ExtIEs", extSpecRows(extIEsHandoverRestrictionList)},
		{"InterSystemMeasurementItem-ExtIEs", extSpecRows(extIEsInterSystemMeasurementItem)},
		{"InterSystemMeasurementParameters-ExtIEs", extSpecRows(extIEsInterSystemMeasurementParameters)},
		{"IntersystemMeasurementConfiguration-ExtIEs", extSpecRows(extIEsIntersystemMeasurementConfiguration)},
		{"LAI-ExtIEs", extSpecRows(extIEsLAI)},
		{"LTE-NTN-TAI-Information-ExtIEs", extSpecRows(extIEsLTENTNTAIInformation)},
		{"LastVisitedEUTRANCellInformation-ExtIEs", extSpecRows(extIEsLastVisitedEUTRANCellInformation)},
		{"LastVisitedPSCellInformation-ExtIEs", extSpecRows(extIEsLastVisitedPSCellInformation)},
		{"ListeningSubframePattern-ExtIEs", extSpecRows(extIEsListeningSubframePattern)},
		{"MutingPatternInformation-ExtIEs", extSpecRows(extIEsMutingPatternInformation)},
		{"NR-CGI-ExtIEs", extSpecRows(extIEsNRCGI)},
		{"NG-eNB-ExtIEs", extSpecRows(extIEsNgENB)},
		{"PSCellInformation-ExtIEs", extSpecRows(extIEsPSCellInformation)},
		{"RAT-RestrictionsItem-ExtIEs", extSpecRows(extIEsRATRestrictionsItem)},
		{"RLFReportInformation-ExtIEs", extSpecRows(extIEsRLFReportInformation)},
		{"RequestType-ExtIEs", extSpecRows(extIEsRequestType)},
		{"RequestedTNLInfo-ExtIEs", extSpecRows(extIEsRequestedTNLInfo)},
		{"SONConfigurationTransfer-ExtIEs", extSpecRows(extIEsSONConfigurationTransfer)},
		{"SONInformationReply-ExtIEs", extSpecRows(extIEsSONInformationReply)},
		{"S-TMSI-ExtIEs", extSpecRows(extIEsSTMSI)},
		{"SecurityContext-ExtIEs", extSpecRows(extIEsSecurityContext)},
		{"SecurityIndication-ExtIEs", extSpecRows(extIEsSecurityIndication)},
		{"ServedGUMMEIsItem-ExtIEs", extSpecRows(extIEsServedGUMMEIsItem)},
		{"SourceNgRanNode-ID-ExtIEs", extSpecRows(extIEsSourceNgRanNodeID)},
		{"SourceeNB-ID-ExtIEs", extSpecRows(extIEsSourceeNBID)},
		{"SourceeNB-ToTargeteNB-TransparentContainer-ExtIEs", extSpecRows(extIEsSourceeNBToTargeteNBTransparentContainer)},
		{"SupportedTAs-Item-ExtIEs", extSpecRows(extIEsSupportedTAItem)},
		{"SynchronisationInformation-ExtIEs", extSpecRows(extIEsSynchronisationInformation)},
		{"TAI-ExtIEs", extSpecRows(extIEsTAI)},
		{"TAI-Broadcast-Item-ExtIEs", extSpecRows(extIEsTAIBroadcastItem)},
		{"TAI-Cancelled-Item-ExtIEs", extSpecRows(extIEsTAICancelledItem)},
		{"TAIItemExtIEs", extSpecRows(extIEsTAIItem)},
		{"TargetNgRanNode-ID-ExtIEs", extSpecRows(extIEsTargetNgRanNodeID)},
		{"TargetRNC-ID-ExtIEs", extSpecRows(extIEsTargetRNCID)},
		{"TargeteNB-ID-ExtIEs", extSpecRows(extIEsTargeteNBID)},
		{"TimeBasedHandoverInformation-ExtIEs", extSpecRows(extIEsTimeBasedHandoverInformation)},
		{"TimeSynchronisationInfo-ExtIEs", extSpecRows(extIEsTimeSynchronisationInfo)},
		{"TraceActivation-ExtIEs", extSpecRows(extIEsTraceActivation)},
		{"Tunnel-Information-ExtIEs", extSpecRows(extIEsTunnelInformation)},
		{"UEAggregate-MaximumBitrates-ExtIEs", extSpecRows(extIEsUEAggregateMaximumBitRate)},
		{"UE-associatedLogicalS1-ConnectionItemExtIEs", extSpecRows(extIEsUEAssociatedLogicalS1ConnectionItem)},
		{"UE-S1AP-ID-pair-ExtIEs", extSpecRows(extIEsUES1APIDPair)},
		{"UESecurityCapabilities-ExtIEs", extSpecRows(extIEsUESecurityCapabilities)},
		{"UserLocationInformation-ExtIEs", extSpecRows(extIEsUserLocationInformation)},
		{"X2TNLConfigurationInfo-ExtIEs", extSpecRows(extIEsX2TNLConfigurationInfo)},
		{"E-RABModifyItemBearerModConfExtIEs", extSpecRows(extIEserabModifyItemBearerModConf)},
	}
}

var knownUntypedExtensions = map[string]string{
	"TraceActivation-ExtIEs/id-MDTConfiguration":     "MDT-Configuration subtree (M1..M7, MDT-Activation, AreaScopeOfMDT); carries no UE id or user-plane TNL",
	"TraceActivation-ExtIEs/id-UEAppLayerMeasConfig": "UEAppLayerMeasConfig subtree (QMC area scope); carries no UE id or user-plane TNL",
}

func asn1Text(t *testing.T) string {
	t.Helper()

	files, err := filepath.Glob("testdata/asn1/*.asn")
	if err != nil || len(files) == 0 {
		t.Fatalf("vendored ASN.1: %v", err)
	}

	var b strings.Builder

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}

		b.WriteString(regexp.MustCompile(`--.*`).ReplaceAllString(string(raw), ""))
		b.WriteString("\n")
	}

	return b.String()
}

type asn1ExtRow struct {
	name     string
	id       ProtocolIEID
	crit     string
	presence string
}

func asn1ExtensionSets(t *testing.T) map[string][]asn1ExtRow {
	t.Helper()

	text := asn1Text(t)

	ids := map[string]ProtocolIEID{}

	for _, m := range regexp.MustCompile(`(id-[\w-]+)\s+ProtocolIE-ID\s*::=\s*(\d+)`).FindAllStringSubmatch(text, -1) {
		v, err := strconv.Atoi(m[2])
		if err != nil {
			t.Fatal(err)
		}

		ids[m[1]] = ProtocolIEID(v)
	}

	sets := map[string][]asn1ExtRow{}
	setRe := regexp.MustCompile(`(?ms)^([A-Za-z][\w-]*)\s+S1AP-PROTOCOL-EXTENSION\s*::=\s*\{(.*?)^\}`)
	rowRe := regexp.MustCompile(`ID\s+(id-[\w-]+)\s+CRITICALITY\s+(\w+)\s+EXTENSION\s+[\w-]+\s+PRESENCE\s+(\w+)`)

	for _, m := range setRe.FindAllStringSubmatch(text, -1) {
		rows := []asn1ExtRow{}

		for _, r := range rowRe.FindAllStringSubmatch(m[2], -1) {
			id, ok := ids[r[1]]
			if !ok {
				t.Fatalf("%s: no ProtocolIE-ID for %s", m[1], r[1])
			}

			rows = append(rows, asn1ExtRow{name: r[1], id: id, crit: r[2], presence: r[3]})
		}

		sets[m[1]] = rows
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
