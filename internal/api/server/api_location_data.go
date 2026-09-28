// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"time"

	"github.com/ellanetworks/core/internal/lmf/models"
	coremodels "github.com/ellanetworks/core/internal/models"
)

// LocationData is the spec-shaped location response, modelled on the TS 29.572
// Nlmf_Location LocationData type (§6.1.6.2.3). It replaces the ad-hoc internal
// LocationResult on the wire.
//
// Deviations from the SBI schema (documented, PR1): the response is wrapped in
// the platform's standard {"result": ...} envelope; RadioMeasurements is a
// non-standard vendor extension.
type LocationData struct {
	LocationEstimate            GeographicArea                  `json:"locationEstimate"`
	AgeOfLocationEstimate       *int32                          `json:"ageOfLocationEstimate,omitempty"`
	TimestampOfLocationEstimate *string                         `json:"timestampOfLocationEstimate,omitempty"`
	PositioningDataList         []PositioningMethodAndUsage     `json:"positioningDataList,omitempty"`
	GnssPositioningDataList     []GnssPositioningMethodAndUsage `json:"gnssPositioningDataList,omitempty"`
	LocNcgi                     *LocNcgi                        `json:"ncgi,omitempty"`
	LocEcgi                     *LocEcgi                        `json:"ecgi,omitempty"`
	Altitude                    *float64                        `json:"altitude,omitempty"`
	RadioMeasurements           []RadioMeasurement              `json:"radioMeasurements,omitempty"`
}

// GeographicArea is a subset of the TS 29.572 GeographicArea discriminated by
// shape. It emits POINT, POINT_UNCERTAINTY_CIRCLE, POINT_UNCERTAINTY_ELLIPSE,
// POINT_ALTITUDE and POINT_ALTITUDE_UNCERTAINTY.
type GeographicArea struct {
	Shape               string              `json:"shape"`
	Point               GeographicalCoord   `json:"point"`
	Uncertainty         *float64            `json:"uncertainty,omitempty"`
	UncertaintyEllipse  *UncertaintyEllipse `json:"uncertaintyEllipse,omitempty"`
	Altitude            *float64            `json:"altitude,omitempty"`
	UncertaintyAltitude *float64            `json:"uncertaintyAltitude,omitempty"`
	Confidence          *int32              `json:"confidence,omitempty"`
}

type UncertaintyEllipse struct {
	SemiMajor        float64 `json:"semiMajor"`
	SemiMinor        float64 `json:"semiMinor"`
	OrientationMajor int32   `json:"orientationMajor"`
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

type RadioMeasurement struct {
	Source            string            `json:"source"`
	RAT               string            `json:"rat"`
	Serving           bool              `json:"serving"`
	PCI               *int64            `json:"pci,omitempty"`
	ARFCN             *int64            `json:"arfcn,omitempty"`
	ARFCNType         string            `json:"arfcnType,omitempty"`
	Ncgi              *LocNcgi          `json:"ncgi,omitempty"`
	Ecgi              *LocEcgi          `json:"ecgi,omitempty"`
	RSRP              *float64          `json:"rsrp,omitempty"`
	RSRQ              *float64          `json:"rsrq,omitempty"`
	SSRSRP            *float64          `json:"ssRsrp,omitempty"`
	SSRSRQ            *float64          `json:"ssRsrq,omitempty"`
	CSIRSRP           *float64          `json:"csiRsrp,omitempty"`
	CSIRSRQ           *float64          `json:"csiRsrq,omitempty"`
	SSBBeams          []BeamMeasurement `json:"ssbBeams,omitempty"`
	CSIRSBeams        []BeamMeasurement `json:"csiRsBeams,omitempty"`
	TimingAdvance     *int64            `json:"timingAdvance,omitempty"`
	NRTimingAdvance   *int64            `json:"nrTimingAdvance,omitempty"`
	UERxTxTimeDiff    *int64            `json:"ueRxTxTimeDiff,omitempty"`
	AoAAzimuthDegrees *float64          `json:"aoaAzimuthDegrees,omitempty"`
	AoAZenithDegrees  *float64          `json:"aoaZenithDegrees,omitempty"`
	DistanceMeters    *float64          `json:"distanceMeters,omitempty"`
}

type BeamMeasurement struct {
	Index int64    `json:"index"`
	RSRP  *float64 `json:"rsrp,omitempty"`
	RSRQ  *float64 `json:"rsrq,omitempty"`
}

// Positioning method / mode / usage enum values (TS 29.572).
const (
	posMethodCellID = "CELLID"
	posMethodECID   = "ECID"
	posMethodNRECID = "NR_ECID"
	gnssGPS         = "GPS"

	posModeUEAssisted                = "UE_ASSISTED"
	posModeUEBased                   = "UE_BASED"
	posModeConvention                = "CONVENTIONAL"
	gadShapePoint                    = "POINT"
	gadShapeCircle                   = "POINT_UNCERTAINTY_CIRCLE"
	gadShapeEllipse                  = "POINT_UNCERTAINTY_ELLIPSE"
	gadShapePointAltitude            = "POINT_ALTITUDE"
	gadShapePointAltitudeUncertainty = "POINT_ALTITUDE_UNCERTAINTY"
)

// toLocationData maps the LMF's internal result onto the spec-shaped response.
func toLocationData(r *models.LocationResult) *LocationData {
	if r == nil {
		return nil
	}

	out := &LocationData{}

	if r.Estimate != nil {
		out.LocationEstimate, out.Altitude = toGeographicArea(r.Estimate)
	}

	// Method + usage.
	for _, a := range r.Positioning {
		if a.Method == models.PositioningMethodGNSS {
			out.GnssPositioningDataList = append(out.GnssPositioningDataList, GnssPositioningMethodAndUsage{
				Mode: positioningMode(a.Mode), Gnss: gnssGPS, Usage: string(a.Usage),
			})

			continue
		}

		out.PositioningDataList = append(out.PositioningDataList, PositioningMethodAndUsage{
			Method: positioningMethod(a.Method), Mode: positioningMode(a.Mode), Usage: string(a.Usage),
		})
	}

	// Cell identities.
	if r.NCGI != nil && r.NCGI.PlmnID != nil {
		out.LocNcgi = toLocNcgi(r.NCGI)
	}

	if r.ECGI != nil && r.ECGI.PlmnID != nil {
		out.LocEcgi = toLocEcgi(r.ECGI)
	}

	if r.AgeOfLocationInfo > 0 {
		age := r.AgeOfLocationInfo
		out.AgeOfLocationEstimate = &age
	}

	if r.UeLocationTimestamp != nil {
		ts := r.UeLocationTimestamp.Format(time.RFC3339)
		out.TimestampOfLocationEstimate = &ts
	}

	for _, m := range r.Measurements {
		out.RadioMeasurements = append(out.RadioMeasurements, toRadioMeasurement(m))
	}

	return out
}

func toRadioMeasurement(m models.CellMeasurement) RadioMeasurement {
	out := RadioMeasurement{
		Source:            string(m.Source),
		RAT:               string(m.RAT),
		Serving:           m.Serving,
		PCI:               m.PCI,
		ARFCN:             m.ARFCN,
		ARFCNType:         string(m.ARFCNType),
		RSRP:              m.RSRP,
		RSRQ:              m.RSRQ,
		SSRSRP:            m.SSRSRP,
		SSRSRQ:            m.SSRSRQ,
		CSIRSRP:           m.CSIRSRP,
		CSIRSRQ:           m.CSIRSRQ,
		SSBBeams:          toBeams(m.SSBBeams),
		CSIRSBeams:        toBeams(m.CSIRSBeams),
		TimingAdvance:     m.TimingAdvance,
		NRTimingAdvance:   m.NRTimingAdvance,
		UERxTxTimeDiff:    m.UERxTxTimeDiff,
		AoAAzimuthDegrees: m.AoAAzimuthDegrees,
		AoAZenithDegrees:  m.AoAZenithDegrees,
		DistanceMeters:    m.DistanceMeters,
	}

	if m.NCGI != nil && m.NCGI.PlmnID != nil {
		out.Ncgi = toLocNcgi(m.NCGI)
	}

	if m.ECGI != nil && m.ECGI.PlmnID != nil {
		out.Ecgi = toLocEcgi(m.ECGI)
	}

	return out
}

func toBeams(beams []models.BeamMeasurement) []BeamMeasurement {
	if len(beams) == 0 {
		return nil
	}

	out := make([]BeamMeasurement, len(beams))
	for i, b := range beams {
		out[i] = BeamMeasurement{Index: b.Index, RSRP: b.RSRP, RSRQ: b.RSRQ}
	}

	return out
}

func toLocNcgi(n *coremodels.Ncgi) *LocNcgi {
	return &LocNcgi{PlmnID: LocPlmn{Mcc: n.PlmnID.Mcc, Mnc: n.PlmnID.Mnc}, NrCellID: n.NrCellID}
}

func toLocEcgi(e *coremodels.Ecgi) *LocEcgi {
	return &LocEcgi{PlmnID: LocPlmn{Mcc: e.PlmnID.Mcc, Mnc: e.PlmnID.Mnc}, EutraCellID: e.EutraCellID}
}

func toGeographicArea(e *models.GeographicEstimate) (GeographicArea, *float64) {
	area := GeographicArea{
		Shape: gadShapePoint,
		Point: GeographicalCoord{Lat: e.LatitudeDegrees, Lon: e.LongitudeDegrees},
	}

	hasEllipse := e.UncertaintyEllipse != nil && e.ConfidencePercent != nil

	switch {
	case hasEllipse && e.AltitudeMeters != nil && e.UncertaintyAltitudeMeters != nil:
		area.Shape = gadShapePointAltitudeUncertainty
		area.Altitude = e.AltitudeMeters
		area.UncertaintyEllipse = toUncertaintyEllipse(e.UncertaintyEllipse)
		area.UncertaintyAltitude = e.UncertaintyAltitudeMeters
		area.Confidence = e.ConfidencePercent

		return area, nil
	case hasEllipse:
		area.Shape = gadShapeEllipse
		area.UncertaintyEllipse = toUncertaintyEllipse(e.UncertaintyEllipse)
		area.Confidence = e.ConfidencePercent

		return area, e.AltitudeMeters
	case e.UncertaintyRadiusMeters != nil:
		area.Shape = gadShapeCircle
		area.Uncertainty = e.UncertaintyRadiusMeters

		return area, e.AltitudeMeters
	case e.UncertaintyEllipse != nil:
		radius := e.UncertaintyEllipse.SemiMajorMeters
		area.Shape = gadShapeCircle
		area.Uncertainty = &radius

		return area, e.AltitudeMeters
	case e.AltitudeMeters != nil:
		area.Shape = gadShapePointAltitude
		area.Altitude = e.AltitudeMeters

		return area, nil
	default:
		return area, nil
	}
}

func toUncertaintyEllipse(e *models.UncertaintyEllipse) *UncertaintyEllipse {
	return &UncertaintyEllipse{
		SemiMajor:        e.SemiMajorMeters,
		SemiMinor:        e.SemiMinorMeters,
		OrientationMajor: e.OrientationMajorDegrees,
	}
}

func positioningMethod(m models.PositioningMethod) string {
	switch m {
	case models.PositioningMethodNRECID:
		return posMethodNRECID
	case models.PositioningMethodECID:
		return posMethodECID
	default:
		return posMethodCellID
	}
}

func positioningMode(m models.PositioningMode) string {
	switch m {
	case models.PositioningModeUEBased:
		return posModeUEBased
	case models.PositioningModeUEAssisted:
		return posModeUEAssisted
	default:
		return posModeConvention
	}
}
