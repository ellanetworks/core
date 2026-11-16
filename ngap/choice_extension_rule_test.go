// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per/ap/apimpl"
)

func TestNGResetWithUnknownResetTypeAlternativeIsRejected(t *testing.T) {
	for _, inner := range []Criticality{CriticalityIgnore, CriticalityNotify, CriticalityReject} {
		value := container(t, ieField{
			ID:          IDResetType,
			Criticality: CriticalityReject,
			Value:       apimpl.RawValue(choiceExtensionValueCrit(t, resetTypeAlternatives, resetTypeChoiceExtensions, 9999, inner)),
		})

		decoded, err := DecodeNGReset(value)
		if err != nil {
			t.Fatalf("%v: decode: %v", inner, err)
		}

		if decoded.ResetType.ChoiceExtension == nil || decoded.ResetType.ChoiceExtension.ID != 9999 {
			t.Fatalf("%v: decoded ResetType = %+v, want the unknown alternative kept", inner, decoded.ResetType)
		}

		_, err = ParseNGReset(value)

		var ase *AbstractSyntaxError
		if !errors.As(err, &ase) {
			t.Fatalf("%v: parse = %v, want *AbstractSyntaxError", inner, err)
		}

		want := IDResetType
		if inner == CriticalityReject {
			want = 9999
		}

		if len(ase.IEs) != 1 || ase.IEs[0].IEID != want || ase.IEs[0].IECriticality != CriticalityReject {
			t.Errorf("%v: diagnostics = %+v, want one reject entry for %v", inner, ase.IEs, want)
		}
	}
}
