// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per/asn1spec"
)

type choiceCase struct {
	asn1 string
	typ  reflect.Type
}

func choiceCases() []choiceCase {
	return []choiceCase{
		{"AMFPagingTarget", reflect.TypeFor[AMFPagingTarget]()},
		{"Area-ID", reflect.TypeFor[AreaID]()},
		{"AreaScopeOfMDT-EUTRA", reflect.TypeFor[AreaScopeOfMDTEUTRA]()},
		{"AreaScopeOfMDT-NR", reflect.TypeFor[AreaScopeOfMDTNR]()},
		{"AreaScopeOfQMC", reflect.TypeFor[AreaScopeOfQMC]()},
		{"BroadcastCancelledAreaList", reflect.TypeFor[BroadcastCancelledAreaList]()},
		{"BroadcastCompletedAreaList", reflect.TypeFor[BroadcastCompletedAreaList]()},
		{"CPTransportLayerInformation", reflect.TypeFor[CPTransportLayerInformation]()},
		{"CandidateCell", reflect.TypeFor[CandidateCell]()},
		{"Cause", reflect.TypeFor[Cause]()},
		{"CellIDListForRestart", reflect.TypeFor[CellIDListForRestart]()},
		{"ClockQualityDetailLevel", reflect.TypeFor[ClockQualityDetailLevel]()},
		{"ECNMarkingorCongestionInformationReportingRequest", reflect.TypeFor[ECNMarkingorCongestionInformationReportingRequest]()},
		{"ENB-ID", reflect.TypeFor[ENBID]()},
		{"EventTrigger", reflect.TypeFor[EventTrigger]()},
		{"GNB-ID", reflect.TypeFor[GNBID]()},
		{"GlobalRANNodeID", reflect.TypeFor[GlobalRANNodeID]()},
		{"InterSystemHandoverReportType", reflect.TypeFor[InterSystemHandoverReportType]()},
		{"IntersystemSONInformation", reflect.TypeFor[IntersystemSONInformation]()},
		{"IntersystemSONInformationReply", reflect.TypeFor[IntersystemSONInformationReply]()},
		{"IntersystemSONInformationReport", reflect.TypeFor[IntersystemSONInformationReport]()},
		{"IntersystemSONInformationRequest", reflect.TypeFor[IntersystemSONInformationRequest]()},
		{"IntersystemSONTransferType", reflect.TypeFor[IntersystemSONTransferType]()},
		{"LastVisitedCellInformation", reflect.TypeFor[LastVisitedCellInformation]()},
		{"LoggedMDTTrigger", reflect.TypeFor[LoggedMDTTrigger]()},
		{"M1ThresholdType", reflect.TypeFor[M1ThresholdType]()},
		{"MBS-ServiceArea", reflect.TypeFor[MBSServiceArea]()},
		{"MDT-AlignmentInfo", reflect.TypeFor[MDTAlignmentInfo]()},
		{"MDTModeNr", reflect.TypeFor[MDTModeNr]()},
		{"MRB-ProgressInformation", reflect.TypeFor[MRBProgressInformation]()},
		{"MeasurementThresholdL1LoggedMDT", reflect.TypeFor[MeasurementThresholdL1LoggedMDT]()},
		{"N3IWF-ID", reflect.TypeFor[N3IWFID]()},
		{"NGRAN-CGI", reflect.TypeFor[NGRANCGI]()},
		{"NPN-AccessInformation", reflect.TypeFor[NPNAccessInformation]()},
		{"NPN-MobilityInformation", reflect.TypeFor[NPNMobilityInformation]()},
		{"NPN-PagingAssistanceInformation", reflect.TypeFor[NPNPagingAssistanceInformation]()},
		{"NPN-Support", reflect.TypeFor[NPNSupport]()},
		{"NTN-Geographical-Area-Item", reflect.TypeFor[NTNGeographicalAreaItem]()},
		{"NgENB-ID", reflect.TypeFor[NgENBID]()},
		{"OverloadResponse", reflect.TypeFor[OverloadResponse]()},
		{"PWSFailedCellIDList", reflect.TypeFor[PWSFailedCellIDList]()},
		{"PeriodicityRange", reflect.TypeFor[PeriodicityRange]()},
		{"QosCharacteristics", reflect.TypeFor[QosCharacteristics]()},
		{"RANfeedbacktype", reflect.TypeFor[RANfeedbacktype]()},
		{"ReportType", reflect.TypeFor[ReportType]()},
		{"ReportingSystem", reflect.TypeFor[ReportingSystem]()},
		{"ResetType", reflect.TypeFor[ResetType]()},
		{"ResourceStatusReportingSystem", reflect.TypeFor[ResourceStatusReportingSystem]()},
		{"SensorNameConfig", reflect.TypeFor[SensorNameConfig]()},
		{"SourceNodeID", reflect.TypeFor[SourceNodeID]()},
		{"TNGF-ID", reflect.TypeFor[TNGFID]()},
		{"TWIF-ID", reflect.TypeFor[TWIFID]()},
		{"TargetID", reflect.TypeFor[TargetID]()},
		{"UEHistoryInformationFromTheUE", reflect.TypeFor[UEHistoryInformationFromTheUE]()},
		{"UEIdentityIndexValue", reflect.TypeFor[UEIdentityIndexValue]()},
		{"UE-NGAP-IDs", reflect.TypeFor[UENGAPIDs]()},
		{"UEPagingIdentity", reflect.TypeFor[UEPagingIdentity]()},
		{"UERLFReportContainer", reflect.TypeFor[UERLFReportContainer]()},
		{"UPTransportLayerInformation", reflect.TypeFor[UPTransportLayerInformation]()},
		{"UserLocationInformation", reflect.TypeFor[UserLocationInformation]()},
		{"UserLocationInformationW-AGF", reflect.TypeFor[UserLocationInformationWAGF]()},
		{"W-AGF-ID", reflect.TypeFor[WAGFID]()},
		{"WarningAreaList", reflect.TypeFor[WarningAreaList]()},
	}
}

type choiceField struct {
	field reflect.StructField
	index int
	ext   bool
	raw   bool
	typed bool
	row   tableRowSpec
}

func choiceFields(t *testing.T, typ reflect.Type) (fields []choiceField, extalt bool) {
	t.Helper()

	for i := range typ.NumField() {
		f := typ.Field(i)

		cf := choiceField{field: f, index: -1}

		for _, o := range strings.Split(f.Tag.Get("per"), ",") {
			switch {
			case o == "extalt":
				extalt = true
			case o == "ext":
				cf.ext = true
			case o == "extie":
				cf.raw = true
			case strings.HasPrefix(o, "extie:"):
				cf.typed = true
				cf.row.ID = protocolIEIDNamed(t, strings.TrimPrefix(o, "extie:"))
			case o == "crit:reject":
				cf.row.Criticality = CriticalityReject
			case o == "crit:ignore":
				cf.row.Criticality = CriticalityIgnore
			case o == "crit:notify":
				cf.row.Criticality = CriticalityNotify
			case strings.HasPrefix(o, "choice:"):
				n, err := strconv.Atoi(strings.TrimPrefix(o, "choice:"))
				if err != nil {
					t.Fatal(err)
				}

				cf.index = n
			}
		}

		if cf.index >= 0 {
			fields = append(fields, cf)
		}
	}

	return fields, extalt
}

func protocolIEIDNamed(t *testing.T, name string) ProtocolIEID {
	t.Helper()

	for id, n := range protocolIENames {
		if "ID"+strings.NewReplacer("-", "", "_", "").Replace(n) == name {
			return id
		}
	}

	t.Fatalf("no ProtocolIE-ID named %s", name)

	return 0
}

func normalizeName(s string) string {
	return strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(s))
}

func alternativeMatches(f reflect.StructField, c *asn1spec.Component) bool {
	elem := f.Type
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}

	return normalizeName(f.Name) == normalizeName(c.Name) ||
		(c.Type.Kind == asn1spec.Reference && normalizeName(elem.Name()) == normalizeName(c.Type.Name))
}

func singleContainerSet(s *asn1spec.Spec, ty *asn1spec.Type) (*asn1spec.ObjectSet, bool) {
	for range 8 {
		if ty.Kind != asn1spec.Reference {
			return nil, false
		}

		if ty.Name == "ProtocolIE-SingleContainer" {
			if len(ty.Args) != 1 || ty.Args[0].Set == nil {
				return nil, false
			}

			return ty.Args[0].Set, true
		}

		ta := s.Type(ty.Name)
		if ta == nil {
			return nil, false
		}

		ty = ta.Type
	}

	return nil, false
}

func singleContainerRows(t *testing.T, set *asn1spec.ObjectSet) []tableRowSpec {
	t.Helper()

	s := vendoredSpec(t)

	objs, err := s.Objects(set)
	if err != nil {
		t.Fatal(err)
	}

	crits := map[string]Criticality{"reject": CriticalityReject, "ignore": CriticalityIgnore, "notify": CriticalityNotify}

	var out []tableRowSpec

	for _, o := range objs {
		idv, _ := o.Value("&id")

		id, err := s.Int(idv)
		if err != nil {
			t.Fatal(err)
		}

		crit, _ := o.Value("&criticality")

		out = append(out, tableRowSpec{ID: ProtocolIEID(id), Criticality: crits[crit.Ref]})
	}

	return out
}

func TestChoiceAlternativesMatchASN1(t *testing.T) {
	s := vendoredSpec(t)

	for _, c := range choiceCases() {
		t.Run(c.asn1, func(t *testing.T) {
			ta := s.Type(c.asn1)
			if ta == nil || ta.Type.Kind != asn1spec.Choice {
				t.Fatalf("%s is not an ASN.1 CHOICE", c.asn1)
			}

			var root, ext []*asn1spec.Component

			for _, comp := range ta.Type.Components {
				if comp.Extension {
					ext = append(ext, comp)
				} else {
					root = append(root, comp)
				}
			}

			fields, extalt := choiceFields(t, c.typ)

			if extalt != ta.Type.Extensible {
				t.Errorf("extalt field %v, spec extensible %v", extalt, ta.Type.Extensible)
			}

			seenRoot := make([]bool, len(root))
			seenExt := make([]bool, len(ext))
			rawAt := map[*asn1spec.Component]bool{}
			typedAt := map[*asn1spec.Component][]tableRowSpec{}

			for _, f := range fields {
				alts, seen := root, seenRoot
				if f.ext {
					alts, seen = ext, seenExt
				}

				if f.index >= len(alts) {
					t.Errorf("%s: alternative %d (ext %v) does not exist in the spec", f.field.Name, f.index, f.ext)

					continue
				}

				comp := alts[f.index]
				seen[f.index] = true

				if f.raw || f.typed {
					if _, ok := singleContainerSet(s, comp.Type); !ok {
						t.Errorf("%s: choice-Extensions field at %d, spec alternative %s is %s", f.field.Name, f.index, comp.Name, comp.Type)

						continue
					}

					if f.raw {
						rawAt[comp] = true
					} else {
						typedAt[comp] = append(typedAt[comp], f.row)
					}

					continue
				}

				if !alternativeMatches(f.field, comp) {
					t.Errorf("%s (%v) at %d, spec alternative is %s %s", f.field.Name, f.field.Type, f.index, comp.Name, comp.Type)
				}
			}

			for i, ok := range seenRoot {
				if !ok {
					t.Errorf("root alternative %s is not modeled", root[i].Name)
				}
			}

			for i, ok := range seenExt {
				if !ok {
					t.Errorf("extension alternative %s is not modeled", ext[i].Name)
				}
			}

			for _, comp := range ta.Type.Components {
				set, ok := singleContainerSet(s, comp.Type)
				if !ok {
					continue
				}

				if !rawAt[comp] {
					t.Errorf("%s: no raw field for unknown choice-Extensions members", comp.Name)
				}

				if want := singleContainerRows(t, set); !slices.Equal(typedAt[comp], want) {
					t.Errorf("%s: typed choice-Extensions %v, spec %v", comp.Name, typedAt[comp], want)
				}
			}
		})
	}
}

func TestChoiceCasesAreComplete(t *testing.T) {
	cases := choiceCases()

	types := make([]reflect.Type, len(cases))
	for i, c := range cases {
		types[i] = c.typ
	}

	checkComplete(t, "CHOICE", scannedDecls(t).choices, types)
}

func TestPDUChoiceMatchesASN1(t *testing.T) {
	ta := vendoredSpec(t).Type("NGAP-PDU")
	if ta == nil {
		t.Fatal("NGAP-PDU not found")
	}

	want := map[string]int{"initiatingMessage": pduInitiatingMessage, "successfulOutcome": pduSuccessfulOutcome, "unsuccessfulOutcome": pduUnsuccessfulOutcome}

	root := 0

	for _, comp := range ta.Type.Components {
		if comp.Extension {
			continue
		}

		if idx, ok := want[comp.Name]; !ok || idx != root {
			t.Errorf("NGAP-PDU alternative %d is %s", root, comp.Name)
		}

		root++
	}

	if root != len(want) || !ta.Type.Extensible {
		t.Errorf("NGAP-PDU has %d root alternatives (extensible %v), want %d and extensible", root, ta.Type.Extensible, len(want))
	}
}
