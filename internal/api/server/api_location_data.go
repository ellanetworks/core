// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"time"

	"github.com/ellanetworks/core/internal/lmf/models"
)

// LocationData is the spec-shaped location response, modelled on the TS 29.572
// Nlmf_Location LocationData type (§6.1.6.2.3). It replaces the ad-hoc internal
// LocationResult on the wire.
//
// Deviations from the SBI schema (documented, PR1): the response is wrapped in
// the platform's standard {"result": ...} envelope; SupplementaryMeasurements
// is a non-standard vendor extension returned only when ?verbose=true;
// locationEstimate is currently always a point + circular uncertainty (the
// uncertainty-ellipse form is a follow-up).
type LocationData struct {
	LocationEstimate            *GeographicArea                 `json:"locationEstimate,omitempty"`
	AccuracyFulfilmentIndicator string                          `json:"accuracyFulfilmentIndicator,omitempty"`
	AgeOfLocationEstimate       *int32                          `json:"ageOfLocationEstimate,omitempty"`
	TimestampOfLocationEstimate *string                         `json:"timestampOfLocationEstimate,omitempty"`
	PositioningDataList         []PositioningMethodAndUsage     `json:"positioningDataList,omitempty"`
	GnssPositioningDataList     []GnssPositioningMethodAndUsage `json:"gnssPositioningDataList,omitempty"`
	LocNcgi                     *LocNcgi                        `json:"ncgi,omitempty"`
	LocEcgi                     *LocEcgi                        `json:"ecgi,omitempty"`
	Altitude                    *float64                        `json:"altitude,omitempty"`
	SupplementaryMeasurements   *SupplementaryMeasurements      `json:"supplementaryMeasurements,omitempty"`
}

// GeographicArea is a subset of the TS 29.572 GeographicArea discriminated by
// shape. PR1 emits POINT and POINT_UNCERTAINTY_CIRCLE.
type GeographicArea struct {
	Shape       string             `json:"shape"`
	Point       *GeographicalCoord `json:"point,omitempty"`
	Uncertainty *float64           `json:"uncertainty,omitempty"`
}

// GeographicalCoord holds WGS-84 decimal degrees.
type GeographicalCoord struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// PositioningMethodAndUsage indicates the method used and its outcome
// (TS 29.572 §6.1.6.2.15).
type PositioningMethodAndUsage struct {
	Method string `json:"method"`
	Mode   string `json:"mode"`
	Usage  string `json:"usage"`
}

type GnssPositioningMethodAndUsage struct {
	Mode  string `json:"mode"`
	Gnss  string `json:"gnss"`
	Usage string `json:"usage"`
}

// LocPlmn / LocNcgi / LocEcgi are SBI (lowerCamelCase) renderings of the cell
// identities (TS 29.571).
type LocPlmn struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type LocNcgi struct {
	PlmnID   LocPlmn `json:"plmnId"`
	NrCellID string  `json:"nrCellId"`
}

type LocEcgi struct {
	PlmnID      LocPlmn `json:"plmnId"`
	EutraCellID string  `json:"eutraCellId"`
}

// SupplementaryMeasurements carries the raw NRPPa measurements. Non-standard;
// returned only with ?verbose=true for debugging.
type SupplementaryMeasurements struct {
	RSRP              *int32   `json:"rsrp,omitempty"`
	RSRQ              *int32   `json:"rsrq,omitempty"`
	TA                *int32   `json:"timingAdvance,omitempty"`
	SSRSRP            *int32   `json:"ssRsrp,omitempty"`
	SSRSRQ            *int32   `json:"ssRsrq,omitempty"`
	CSIRSRP           *int32   `json:"csiRsrp,omitempty"`
	CSIRSRQ           *int32   `json:"csiRsrq,omitempty"`
	NRTimingAdvance   *int32   `json:"nrTimingAdvance,omitempty"`
	UERxTxTimeDiff    *int32   `json:"ueRxTxTimeDiff,omitempty"`
	AoAAzimuthDegrees *float64 `json:"aoaAzimuthDegrees,omitempty"`
	AoAZenithDegrees  *float64 `json:"aoaZenithDegrees,omitempty"`
	DistanceMeters    *float64 `json:"distanceMeters,omitempty"`
}

// Positioning method / mode / usage enum values (TS 29.572).
const (
	posMethodCellID = "CELLID"
	posMethodECID   = "ECID"
	posMethodNRECID = "NR_ECID"
	gnssGPS         = "GPS"

	posModeUEAssisted = "UE_ASSISTED"
	posModeUEBased    = "UE_BASED"
	posModeConvention = "CONVENTIONAL"
	usageSuccessUsed  = "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION"
	gadShapePoint     = "POINT"
	gadShapeCircle    = "POINT_UNCERTAINTY_CIRCLE"
	accuracyFulfilled = "REQUESTED_ACCURACY_FULFILLED"
)

const meters = 0.01

// toLocationData maps the LMF's internal result onto the spec-shaped response.
func toLocationData(r *models.LocationResult, verbose bool) *LocationData {
	if r == nil {
		return nil
	}

	out := &LocationData{
		AccuracyFulfilmentIndicator: accuracyFulfilled,
	}

	// Geographic estimate (point + optional circular uncertainty).
	point := &GeographicalCoord{
		Lat: float64(r.Latitude) / 1e7,
		Lon: float64(r.Longitude) / 1e7,
	}

	area := &GeographicArea{Shape: gadShapePoint, Point: point}

	if r.HorizontalAccuracy > 0 {
		unc := float64(r.HorizontalAccuracy)
		area.Shape = gadShapeCircle
		area.Uncertainty = &unc
	}

	if r.Altitude != 0 {
		alt := float64(r.Altitude) * meters
		out.Altitude = &alt
	}

	out.LocationEstimate = area

	// Method + usage.
	if r.Shape == models.GADEllipsoidalPoint {
		out.GnssPositioningDataList = []GnssPositioningMethodAndUsage{
			{Mode: posModeUEBased, Gnss: gnssGPS, Usage: usageSuccessUsed},
		}
	} else {
		method, mode := methodAndMode(r)
		out.PositioningDataList = []PositioningMethodAndUsage{
			{Method: method, Mode: mode, Usage: usageSuccessUsed},
		}
	}

	// Cell identities.
	if r.NCGI != nil && r.NCGI.PlmnID != nil {
		out.LocNcgi = &LocNcgi{PlmnID: LocPlmn{Mcc: r.NCGI.PlmnID.Mcc, Mnc: r.NCGI.PlmnID.Mnc}, NrCellID: r.NCGI.NrCellID}
	}

	if r.ECGI != nil && r.ECGI.PlmnID != nil {
		out.LocEcgi = &LocEcgi{PlmnID: LocPlmn{Mcc: r.ECGI.PlmnID.Mcc, Mnc: r.ECGI.PlmnID.Mnc}, EutraCellID: r.ECGI.EutraCellID}
	}

	if r.AgeOfLocationInfo > 0 {
		age := r.AgeOfLocationInfo
		out.AgeOfLocationEstimate = &age
	}

	if r.UeLocationTimestamp != nil {
		ts := r.UeLocationTimestamp.Format(time.RFC3339)
		out.TimestampOfLocationEstimate = &ts
	}

	if verbose {
		out.SupplementaryMeasurements = &SupplementaryMeasurements{
			RSRP:              r.RSRP,
			RSRQ:              r.RSRQ,
			TA:                r.TA,
			SSRSRP:            r.SSRSRP,
			SSRSRQ:            r.SSRSRQ,
			CSIRSRP:           r.CSIRSRP,
			CSIRSRQ:           r.CSIRSRQ,
			NRTimingAdvance:   r.NRTimingAdvance,
			UERxTxTimeDiff:    r.UERxTxTimeDiff,
			AoAAzimuthDegrees: r.AoAAzimuthDegrees,
			AoAZenithDegrees:  r.AoAZenithDegrees,
			DistanceMeters:    r.Distance,
		}
	}

	return out
}

// methodAndMode derives the SBI positioning method + mode from the internal
// result shape and access type.
func methodAndMode(r *models.LocationResult) (method, mode string) {
	switch r.Shape {
	case models.GADECID:
		if r.AccessType == "NR" {
			return posMethodNRECID, posModeUEAssisted
		}

		return posMethodECID, posModeUEAssisted
	default:
		return posMethodCellID, posModeConvention
	}
}
