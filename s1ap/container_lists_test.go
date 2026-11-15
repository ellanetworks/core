// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/asn1spec"
)

type itemListSpec struct {
	item tableRowSpec
	ub   int64
}

func asn1ItemList(t *testing.T, ty *asn1spec.Type) (itemListSpec, bool) {
	t.Helper()

	s := vendoredSpec(t)

	var (
		set *asn1spec.ObjectSet
		ub  *asn1spec.Value
	)

	for range 8 {
		switch {
		case ty.Kind == asn1spec.SequenceOf && ty.Elem.Name == "ProtocolIE-SingleContainer" && ty.Size != nil:
			set, ub = ty.Elem.Args[0].Set, &ty.Size.Root[0].Upper
		case ty.Kind == asn1spec.Reference && ty.Name == "ProtocolIE-ContainerList":
			ub = ty.Args[1].Value
		}

		if set == nil && ty.Kind == asn1spec.Reference {
			for _, a := range ty.Args {
				if a.Set != nil && len(a.Set.Root) == 1 && a.Set.Root[0].Ref != "" && s.ObjectSet(a.Set.Root[0].Ref) != nil {
					set = a.Set
				}
			}
		}

		if set != nil && ub != nil {
			break
		}

		if ty.Kind != asn1spec.Reference {
			return itemListSpec{}, false
		}

		ta := s.Type(ty.Name)
		if ta == nil {
			return itemListSpec{}, false
		}

		ty = ta.Type
	}

	if set == nil || ub == nil {
		return itemListSpec{}, false
	}

	objs, err := s.Objects(set)
	if err != nil {
		t.Fatal(err)
	}

	if len(objs) != 1 {
		t.Fatalf("item IE set has %d members", len(objs))
	}

	n, err := s.Int(*ub)
	if err != nil {
		t.Fatal(err)
	}

	return itemListSpec{item: asn1Row(t, objs[0]), ub: n}, true
}

func wireItems(t *testing.T, value []byte, ub int64) []tableRowSpec {
	t.Helper()

	r := per.NewReader(value)

	var out []tableRowSpec

	err := per.DecodeLength(r, per.Aligned, 1, ub, true, func(count int64) error {
		for range count {
			id, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs)
			if err != nil {
				return err
			}

			c, err := per.DecodeEnumerated(r, per.Aligned, 3, false)
			if err != nil {
				return err
			}

			if _, err := per.DecodeOpenTypeBytes(r, per.Aligned); err != nil {
				return err
			}

			out = append(out, tableRowSpec{ID: ProtocolIEID(id), Criticality: Criticality(c)})
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func checkItems(t *testing.T, name string, spec itemListSpec, value []byte) {
	t.Helper()

	items := wireItems(t, value, spec.ub)
	if len(items) == 0 {
		t.Fatalf("%s: sample list is empty", name)
	}

	for _, it := range items {
		if it.ID != spec.item.ID || it.Criticality != spec.item.Criticality {
			t.Errorf("%s: item sent as %s/%s, spec says %s/%s", name, it.ID, it.Criticality, spec.item.ID, spec.item.Criticality)
		}
	}
}

func TestMessageItemListsMatchASN1(t *testing.T) {
	checked := 0

	for _, c := range messageCases() {
		t.Run(c.asn1, func(t *testing.T) {
			types := map[ProtocolIEID]*asn1spec.Type{}
			for _, o := range asn1Objects(t, c.asn1, "ProtocolIE-Container") {
				types[asn1Row(t, o).ID] = o.Type("&Value")
			}

			b, err := sampleMessage(t, c.typ).Interface().(interface{ Marshal() ([]byte, error) }).Marshal()
			if err != nil {
				t.Fatal(err)
			}

			raw, err := ParseRawPDU(b)
			if err != nil {
				t.Fatal(err)
			}

			for _, ie := range raw.IEs() {
				spec, ok := asn1ItemList(t, types[ie.ID])
				if !ok {
					continue
				}

				checkItems(t, ie.ID.String(), spec, ie.Value)

				checked++
			}
		})
	}

	if checked == 0 {
		t.Fatal("no single-container list was checked")
	}
}

func TestNestedItemListsMatchASN1(t *testing.T) {
	for _, c := range []struct {
		asn1 string
		typ  reflect.Type
	}{
		{"UE-associatedLogicalS1-ConnectionListRes", reflect.TypeFor[UEAssociatedLogicalS1ConnectionListRes]()},
		{"E-RABInformationList", reflect.TypeFor[ERABInformationList]()},
		{"TAIList", reflect.TypeFor[TAIList]()},
		{"E-RABToBeUpdatedList", reflect.TypeFor[ERABToBeUpdatedList]()},
		{"SecondaryRATDataUsageReportList", reflect.TypeFor[SecondaryRATDataUsageReportList]()},
		{"RecommendedCellList", reflect.TypeFor[RecommendedCellList]()},
		{"RecommendedENBList", reflect.TypeFor[RecommendedENBList]()},
		{"E-RABUsageReportList", reflect.TypeFor[ERABUsageReportList]()},
	} {
		t.Run(c.asn1, func(t *testing.T) {
			ta := vendoredSpec(t).Type(c.asn1)
			if ta == nil {
				t.Fatalf("%s not found", c.asn1)
			}

			spec, ok := asn1ItemList(t, ta.Type)
			if !ok {
				t.Fatalf("%s is not a ProtocolIE-SingleContainer list", c.asn1)
			}

			v := reflect.New(c.typ)
			fillSample(t, v.Elem(), 0)

			b, err := per.Marshal(v.Interface().(per.Marshaler))
			if err != nil {
				t.Fatal(err)
			}

			checkItems(t, c.asn1, spec, b)
		})
	}
}

func TestItemListElementTypesMatchASN1(t *testing.T) {
	for _, c := range []struct {
		asn1 string
		typ  reflect.Type
	}{
		{"E-RABToBeSetupListCtxtSUReq", reflect.TypeFor[ERABToBeSetupListCtxtSUReq]()},
		{"E-RABSetupListCtxtSURes", reflect.TypeFor[ERABSetupListCtxtSURes]()},
		{"E-RABToBeSetupListBearerSUReq", reflect.TypeFor[ERABToBeSetupListBearerSUReq]()},
		{"E-RABSetupListBearerSURes", reflect.TypeFor[ERABSetupListBearerSURes]()},
		{"E-RABToBeModifiedListBearerModReq", reflect.TypeFor[ERABToBeModifiedListBearerModReq]()},
		{"E-RABToBeModifiedListBearerModInd", reflect.TypeFor[ERABToBeModifiedListBearerModInd]()},
		{"E-RABNotToBeModifiedListBearerModInd", reflect.TypeFor[ERABNotToBeModifiedListBearerModInd]()},
		{"E-RABToBeSetupListHOReq", reflect.TypeFor[ERABToBeSetupListHOReq]()},
		{"E-RABAdmittedList", reflect.TypeFor[ERABAdmittedList]()},
		{"E-RABSubjecttoDataForwardingList", reflect.TypeFor[ERABSubjecttoDataForwardingList]()},
		{"E-RABToBeSwitchedDLList", reflect.TypeFor[ERABToBeSwitchedDLList]()},
		{"E-RABToBeSwitchedULList", reflect.TypeFor[ERABToBeSwitchedULList]()},
		{"TAIList", reflect.TypeFor[TAIList]()},
		{"E-RABToBeUpdatedList", reflect.TypeFor[ERABToBeUpdatedList]()},
		{"SecondaryRATDataUsageReportList", reflect.TypeFor[SecondaryRATDataUsageReportList]()},
		{"RecommendedCellList", reflect.TypeFor[RecommendedCellList]()},
		{"RecommendedENBList", reflect.TypeFor[RecommendedENBList]()},
		{"E-RABUsageReportList", reflect.TypeFor[ERABUsageReportList]()},
		{"E-RABInformationList", reflect.TypeFor[ERABInformationList]()},
		{"UE-associatedLogicalS1-ConnectionListRes", reflect.TypeFor[UEAssociatedLogicalS1ConnectionListRes]()},
	} {
		t.Run(c.asn1, func(t *testing.T) {
			s := vendoredSpec(t)

			ta := s.Type(c.asn1)
			if ta == nil {
				t.Fatalf("%s not found", c.asn1)
			}

			item := asn1ItemType(t, ta.Type)
			if got := c.typ.Elem().Name(); goTypeName(item) != got {
				t.Errorf("%s items are %s, spec says %s", c.asn1, got, item)
			}
		})
	}
}

func asn1ItemType(t *testing.T, ty *asn1spec.Type) string {
	t.Helper()

	s := vendoredSpec(t)

	for range 8 {
		var set *asn1spec.ObjectSet

		switch {
		case ty.Kind == asn1spec.SequenceOf && ty.Elem.Name == "ProtocolIE-SingleContainer":
			set = ty.Elem.Args[0].Set
		case ty.Kind == asn1spec.Reference:
			for _, a := range ty.Args {
				if a.Set != nil && len(a.Set.Root) == 1 && a.Set.Root[0].Ref != "" && s.ObjectSet(a.Set.Root[0].Ref) != nil {
					set = a.Set
				}
			}
		}

		if set != nil {
			objs, err := s.Objects(set)
			if err != nil {
				t.Fatal(err)
			}

			if len(objs) != 1 {
				t.Fatalf("item IE set has %d members", len(objs))
			}

			return objs[0].Type("&Value").Name
		}

		ta := s.Type(ty.Name)
		if ty.Kind != asn1spec.Reference || ta == nil {
			t.Fatalf("%s is not a ProtocolIE-SingleContainer list", ty.Name)
		}

		ty = ta.Type
	}

	t.Fatal("item list nests too deep")

	return ""
}

func goTypeName(asn1 string) string {
	return strings.ReplaceAll(strings.Replace(asn1, "UE-associated", "UEAssociated", 1), "-", "")
}
