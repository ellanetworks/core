// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"errors"
	"fmt"
	"slices"
)

type MeasurementQuantity string

const (
	MeasurementQuantityCellID             MeasurementQuantity = "cell-ID"
	MeasurementQuantityAngleOfArrival     MeasurementQuantity = "angleOfArrival"
	MeasurementQuantityTimingAdvanceType1 MeasurementQuantity = "timingAdvanceType1"
	MeasurementQuantityTimingAdvanceType2 MeasurementQuantity = "timingAdvanceType2"
	MeasurementQuantityRSRP               MeasurementQuantity = "rSRP"
	MeasurementQuantityRSRQ               MeasurementQuantity = "rSRQ"
	MeasurementQuantitySSRSRP             MeasurementQuantity = "sS-RSRP"
	MeasurementQuantitySSRSRQ             MeasurementQuantity = "sS-RSRQ"
	MeasurementQuantityCSIRSRP            MeasurementQuantity = "cSI-RSRP"
	MeasurementQuantityCSIRSRQ            MeasurementQuantity = "cSI-RSRQ"
	MeasurementQuantityAngleOfArrivalNR   MeasurementQuantity = "angleOfArrivalNR"
	MeasurementQuantityTimingAdvanceNR    MeasurementQuantity = "timingAdvanceNR"
	MeasurementQuantityUERxTxTimeDiff     MeasurementQuantity = "uE-Rx-Tx-Time-Diff"
)

var EUTRAMeasurementQuantities = []MeasurementQuantity{
	MeasurementQuantityCellID,
	MeasurementQuantityAngleOfArrival,
	MeasurementQuantityTimingAdvanceType1,
	MeasurementQuantityTimingAdvanceType2,
	MeasurementQuantityRSRP,
	MeasurementQuantityRSRQ,
}

var NRMeasurementQuantities = []MeasurementQuantity{
	MeasurementQuantityCellID,
	MeasurementQuantitySSRSRP,
	MeasurementQuantitySSRSRQ,
	MeasurementQuantityCSIRSRP,
	MeasurementQuantityCSIRSRQ,
	MeasurementQuantityAngleOfArrivalNR,
	MeasurementQuantityTimingAdvanceNR,
	MeasurementQuantityUERxTxTimeDiff,
}

var defaultNRMeasurementQuantities = []MeasurementQuantity{
	MeasurementQuantitySSRSRP,
	MeasurementQuantitySSRSRQ,
	MeasurementQuantityCSIRSRP,
	MeasurementQuantityCSIRSRQ,
	MeasurementQuantityAngleOfArrivalNR,
	MeasurementQuantityTimingAdvanceNR,
	MeasurementQuantityUERxTxTimeDiff,
}

func DefaultMeasurementQuantities(nr bool) []MeasurementQuantity {
	if nr {
		return slices.Clone(defaultNRMeasurementQuantities)
	}

	return slices.Clone(EUTRAMeasurementQuantities)
}

type MeasurementQuantities struct {
	NR    []MeasurementQuantity `json:"nr,omitempty"`
	EUTRA []MeasurementQuantity `json:"eutra,omitempty"`
}

func (q *MeasurementQuantities) ForRAT(nr bool) []MeasurementQuantity {
	var list []MeasurementQuantity

	switch {
	case q == nil:
	case nr:
		list = q.NR
	default:
		list = q.EUTRA
	}

	if list == nil {
		return DefaultMeasurementQuantities(nr)
	}

	return list
}

var ErrInvalidMeasurementQuantities = errors.New("invalid measurement quantities")

func (q *MeasurementQuantities) Validate() error {
	if q.NR == nil && q.EUTRA == nil {
		return fmt.Errorf("%w: at least one of nr, eutra is required", ErrInvalidMeasurementQuantities)
	}

	if err := validateQuantityList("nr", q.NR, NRMeasurementQuantities); err != nil {
		return err
	}

	return validateQuantityList("eutra", q.EUTRA, EUTRAMeasurementQuantities)
}

func validateQuantityList(rat string, list, allowed []MeasurementQuantity) error {
	if list == nil {
		return nil
	}

	if len(list) == 0 {
		return fmt.Errorf("%w: %s list must not be empty", ErrInvalidMeasurementQuantities, rat)
	}

	seen := make(map[MeasurementQuantity]bool, len(list))

	for _, v := range list {
		if !slices.Contains(allowed, v) {
			return fmt.Errorf("%w: %q is not a valid %s quantity", ErrInvalidMeasurementQuantities, v, rat)
		}

		if seen[v] {
			return fmt.Errorf("%w: %q is listed more than once in %s", ErrInvalidMeasurementQuantities, v, rat)
		}

		seen[v] = true
	}

	return nil
}
