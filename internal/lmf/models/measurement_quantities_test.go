// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"errors"
	"slices"
	"testing"
)

func TestMeasurementQuantitiesValidate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		q     MeasurementQuantities
		valid bool
	}{
		{"zero", MeasurementQuantities{}, false},
		{"nr only", MeasurementQuantities{NR: []MeasurementQuantity{MeasurementQuantitySSRSRP, MeasurementQuantityTimingAdvanceNR}}, true},
		{"nr accepts cell-ID", MeasurementQuantities{NR: []MeasurementQuantity{MeasurementQuantityCellID}}, true},
		{"nr rejects E-UTRA values", MeasurementQuantities{NR: []MeasurementQuantity{MeasurementQuantityRSRP}}, false},
		{"eutra only", MeasurementQuantities{EUTRA: []MeasurementQuantity{MeasurementQuantityRSRP, MeasurementQuantityTimingAdvanceType1}}, true},
		{"empty nr", MeasurementQuantities{NR: []MeasurementQuantity{}}, false},
		{"unknown value", MeasurementQuantities{NR: []MeasurementQuantity{"ssRsrp"}}, false},
		{"nr value in eutra", MeasurementQuantities{EUTRA: []MeasurementQuantity{MeasurementQuantitySSRSRP}}, false},
		{"duplicate", MeasurementQuantities{EUTRA: []MeasurementQuantity{MeasurementQuantityRSRP, MeasurementQuantityRSRP}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.q.Validate()
			if tc.valid && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !tc.valid && !errors.Is(err, ErrInvalidMeasurementQuantities) {
				t.Fatalf("err = %v, want ErrInvalidMeasurementQuantities", err)
			}
		})
	}
}

func TestMeasurementQuantitiesForRAT(t *testing.T) {
	explicit := []MeasurementQuantity{MeasurementQuantityTimingAdvanceNR}

	for _, tc := range []struct {
		name string
		q    *MeasurementQuantities
		nr   bool
		want []MeasurementQuantity
	}{
		{"absent, NR", nil, true, DefaultMeasurementQuantities(true)},
		{"absent, E-UTRA", nil, false, DefaultMeasurementQuantities(false)},
		{"explicit NR", &MeasurementQuantities{NR: explicit}, true, explicit},
		{"NR only, E-UTRA cell", &MeasurementQuantities{NR: explicit}, false, EUTRAMeasurementQuantities},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.q.ForRAT(tc.nr); !slices.Equal(got, tc.want) {
				t.Errorf("ForRAT(%v) = %v, want %v", tc.nr, got, tc.want)
			}
		})
	}
}

func TestDefaultMeasurementQuantitiesAreValid(t *testing.T) {
	q := MeasurementQuantities{NR: DefaultMeasurementQuantities(true), EUTRA: DefaultMeasurementQuantities(false)}
	if err := q.Validate(); err != nil {
		t.Fatalf("defaults do not validate: %v", err)
	}
}
