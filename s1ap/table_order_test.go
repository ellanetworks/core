// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"reflect"
	"slices"
	"testing"
)

func TestTableOrderMatchesASN1(t *testing.T) {
	for _, c := range messageCases() {
		t.Run(c.asn1, func(t *testing.T) {
			want := asn1Rows(t, c.asn1, "ProtocolIE-Container")
			if !slices.Equal(c.rows, want) {
				t.Errorf("table %v\nspec  %v", c.rows, want)
			}
		})
	}
}

func TestMessageCasesAreComplete(t *testing.T) {
	cases := messageCases()

	types := make([]reflect.Type, len(cases))
	for i, c := range cases {
		types[i] = c.typ
	}

	checkComplete(t, "IE table", scannedDecls(t).ieTables, types)
}

func tableIDs[M any](table []ieSpec[M]) []ProtocolIEID {
	ids := make([]ProtocolIEID, len(table))
	for i, spec := range table {
		ids[i] = spec.ID
	}

	return ids
}
