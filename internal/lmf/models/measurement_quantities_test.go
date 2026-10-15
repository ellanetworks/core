// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"errors"
	"testing"
)

func TestMeasurementQuantitiesValidate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		q     MeasurementQuantities
		valid bool
	}{
		{"zero", MeasurementQuantities{}, true},
		{"nr only", MeasurementQuantities{NR: []MeasurementQuantity{MeasurementQuantitySSRSRP, MeasurementQuantityTimingAdvanceNR}}, true},
		{"nr accepts root values", MeasurementQuantities{NR: []MeasurementQuantity{MeasurementQuantityRSRP}}, true},
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
