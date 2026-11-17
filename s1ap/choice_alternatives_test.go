// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"maps"
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
		{"BroadcastCancelledAreaList", reflect.TypeFor[BroadcastCancelledAreaList]()},
		{"BroadcastCompletedAreaList", reflect.TypeFor[BroadcastCompletedAreaList]()},
		{"Cause", reflect.TypeFor[Cause]()},
		{"ENB-ID", reflect.TypeFor[ENBID]()},
		{"EN-DCSONTransferType", reflect.TypeFor[ENDCSONTransferType]()},
		{"Global-RAN-NODE-ID", reflect.TypeFor[GlobalRANNodeID]()},
		{"GNB-Identity", reflect.TypeFor[GNBIdentity]()},
		{"LastVisitedCell-Item", reflect.TypeFor[LastVisitedCellItem]()},
		{"LastVisitedGERANCellInformation", reflect.TypeFor[LastVisitedGERANCellInformation]()},
		{"MMEPagingTarget", reflect.TypeFor[MMEPagingTarget]()},
		{"OverloadResponse", reflect.TypeFor[OverloadResponse]()},
		{"ResetType", reflect.TypeFor[ResetType]()},
		{"SONInformation", reflect.TypeFor[SONInformation]()},
		{"SONInformationReport", reflect.TypeFor[SONInformationReport]()},
		{"SourceNodeID", reflect.TypeFor[SourceNodeID]()},
		{"TargetID", reflect.TypeFor[TargetID]()},
		{"UEPagingID", reflect.TypeFor[UEPagingID]()},
		{"UE-S1AP-IDs", reflect.TypeFor[UES1APIDs]()},
		{"WarningAreaList", reflect.TypeFor[WarningAreaList]()},
	}
}

type choiceField struct {
	field reflect.StructField
	index int
	ext   bool
	raw   bool
}

func choiceFields(t *testing.T, typ reflect.Type) (fields []choiceField, extalt bool) {
	t.Helper()

	for i := range typ.NumField() {
		f := typ.Field(i)
		opts := strings.Split(f.Tag.Get("per"), ",")

		cf := choiceField{field: f, index: -1}

		for _, o := range opts {
			switch {
			case o == "extalt":
				extalt = true
			case o == "ext":
				cf.ext = true
			case o == "extie":
				cf.raw = true
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

func isSingleContainer(s *asn1spec.Spec, ty *asn1spec.Type) bool {
	_, ok := singleContainerSet(s, ty)

	return ok
}

func singleContainerSet(s *asn1spec.Spec, ty *asn1spec.Type) (string, bool) {
	for range 8 {
		if ty.Kind != asn1spec.Reference {
			return "", false
		}

		if ty.Name == "ProtocolIE-SingleContainer" {
			if len(ty.Args) != 1 || ty.Args[0].Set == nil || len(ty.Args[0].Set.Root) != 1 {
				return "", true
			}

			return ty.Args[0].Set.Root[0].Ref, true
		}

		ta := s.Type(ty.Name)
		if ta == nil {
			return "", false
		}

		ty = ta.Type
	}

	return "", false
}

type choiceExtensionCase struct {
	set string
	typ reflect.Type
}

func choiceExtensionCases() []choiceExtensionCase {
	return []choiceExtensionCase{
		{"SONInformation-ExtensionIE", reflect.TypeFor[SONInformationExtension]()},
		{"SourceNodeID-ExtensionIE", reflect.TypeFor[SourceNodeID]()},
	}
}

func TestChoiceCasesAreComplete(t *testing.T) {
	s := vendoredSpec(t)

	sets := map[string]bool{}
	types := map[reflect.Type]bool{}

	for _, c := range choiceExtensionCases() {
		sets[c.set] = true
		types[c.typ] = true
	}

	for _, c := range choiceCases() {
		types[c.typ] = true

		ta := s.Type(c.asn1)
		if ta == nil {
			t.Errorf("%s not found", c.asn1)

			continue
		}

		for _, comp := range ta.Type.Components {
			set, ok := singleContainerSet(s, comp.Type)
			if !ok {
				continue
			}

			if set == "" {
				t.Errorf("%s.%s: cannot name the choice-Extensions set of %s", c.asn1, comp.Name, comp.Type)

				continue
			}

			if !sets[set] {
				t.Errorf("%s.%s: %s has no choiceExtensionCases entry", c.asn1, comp.Name, set)
			}
		}
	}

	checkComplete(t, "CHOICE", scannedDecls(t).choices, slices.Collect(maps.Keys(types)))
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

				if f.raw {
					if !isSingleContainer(s, comp.Type) {
						t.Errorf("%s: choice-Extensions field at %d, spec alternative %s is %s", f.field.Name, f.index, comp.Name, comp.Type)
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
		})
	}
}

func TestPDUChoiceMatchesASN1(t *testing.T) {
	ta := vendoredSpec(t).Type("S1AP-PDU")
	if ta == nil {
		t.Fatal("S1AP-PDU not found")
	}

	want := map[string]int{"initiatingMessage": pduInitiatingMessage, "successfulOutcome": pduSuccessfulOutcome, "unsuccessfulOutcome": pduUnsuccessfulOutcome}

	root := 0

	for _, comp := range ta.Type.Components {
		if comp.Extension {
			continue
		}

		if idx, ok := want[comp.Name]; !ok || idx != root {
			t.Errorf("S1AP-PDU alternative %d is %s", root, comp.Name)
		}

		root++
	}

	if root != len(want) || !ta.Type.Extensible {
		t.Errorf("S1AP-PDU has %d root alternatives (extensible %v), want %d and extensible", root, ta.Type.Extensible, len(want))
	}
}

func extieTag(t *testing.T, f reflect.StructField) (tableRowSpec, bool) {
	t.Helper()

	var (
		row   tableRowSpec
		typed bool
	)

	for _, o := range strings.Split(f.Tag.Get("per"), ",") {
		switch {
		case strings.HasPrefix(o, "extie:"):
			name := strings.TrimPrefix(o, "extie:")

			for id, n := range protocolIENames {
				if "ID"+goIdentifier(n) == name {
					row.ID, typed = id, true
				}
			}

			if !typed {
				t.Fatalf("%s: no ProtocolIE-ID named %s", f.Name, name)
			}
		case o == "crit:reject":
			row.Criticality = CriticalityReject
		case o == "crit:ignore":
			row.Criticality = CriticalityIgnore
		case o == "crit:notify":
			row.Criticality = CriticalityNotify
		}
	}

	return row, typed
}

func TestChoiceExtensionsMatchASN1(t *testing.T) {
	s := vendoredSpec(t)

	for _, c := range choiceExtensionCases() {
		t.Run(c.set, func(t *testing.T) {
			sa := s.ObjectSet(c.set)
			if sa == nil {
				t.Fatalf("%s not found", c.set)
			}

			objs, err := s.Objects(sa.Set)
			if err != nil {
				t.Fatal(err)
			}

			var want []tableRowSpec

			for _, o := range objs {
				row := asn1Row(t, o)
				want = append(want, tableRowSpec{ID: row.ID, Criticality: row.Criticality})
			}

			var got []tableRowSpec

			raw := false

			for i := range c.typ.NumField() {
				f := c.typ.Field(i)
				if !strings.Contains(f.Tag.Get("per"), "extie") {
					continue
				}

				row, typed := extieTag(t, f)
				if typed {
					got = append(got, row)
				} else {
					raw = true
				}
			}

			if !raw {
				t.Errorf("%v has no raw field for unknown %s members", c.typ, c.set)
			}

			if len(got) != len(want) {
				t.Fatalf("typed %v, spec %v", got, want)
			}

			for i := range got {
				if got[i] != want[i] {
					t.Errorf("typed %v, spec %v", got[i], want[i])
				}
			}
		})
	}
}
