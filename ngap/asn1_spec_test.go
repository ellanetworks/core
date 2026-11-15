// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"os"
	"sync"
	"testing"

	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/asn1spec"
)

type tableRowSpec struct {
	ID          ProtocolIEID
	Criticality Criticality
	Presence    ap.Presence
}

var (
	asn1Once sync.Once
	asn1Spec *asn1spec.Spec
	asn1Err  error
)

func vendoredSpec(t *testing.T) *asn1spec.Spec {
	t.Helper()

	asn1Once.Do(func() {
		asn1Spec, asn1Err = asn1spec.ParseFS(os.DirFS("testdata/asn1"), "*.asn")
	})

	if asn1Err != nil {
		t.Fatal(asn1Err)
	}

	return asn1Spec
}

func asn1Rows(t *testing.T, typeName, container string) []tableRowSpec {
	t.Helper()

	s := vendoredSpec(t)

	ta := s.Type(typeName)
	if ta == nil {
		t.Fatalf("ASN.1 type %s not found", typeName)
	}

	var set *asn1spec.ObjectSet

	for _, c := range ta.Type.Components {
		if c.Type.Name == container && len(c.Type.Args) == 1 {
			set = c.Type.Args[0].Set
		}
	}

	if set == nil {
		t.Fatalf("%s has no %s", typeName, container)
	}

	objs, err := s.Objects(set)
	if err != nil {
		t.Fatal(err)
	}

	crits := map[string]Criticality{"reject": CriticalityReject, "ignore": CriticalityIgnore, "notify": CriticalityNotify}
	presences := map[string]ap.Presence{"optional": ap.PresenceOptional, "conditional": ap.PresenceConditional, "mandatory": ap.PresenceMandatory}

	out := make([]tableRowSpec, 0, len(objs))

	for _, o := range objs {
		idv, _ := o.Value("&id")

		id, err := s.Int(idv)
		if err != nil {
			t.Fatal(err)
		}

		crit, _ := o.Value("&criticality")
		pres, _ := o.Value("&presence")

		out = append(out, tableRowSpec{ID: ProtocolIEID(id), Criticality: crits[crit.Ref], Presence: presences[pres.Ref]})
	}

	return out
}

func tableRows[M any](table []ieSpec[M]) []tableRowSpec {
	out := make([]tableRowSpec, len(table))
	for i, r := range table {
		out[i] = tableRowSpec{ID: r.ID, Criticality: r.Criticality, Presence: r.Presence}
	}

	return out
}
