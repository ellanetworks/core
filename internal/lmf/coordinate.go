// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/models"
	coremodels "github.com/ellanetworks/core/internal/models"
)

// ErrNoLocationEstimate indicates the LMF could not anchor a geographic
// location estimate: neither the RAN (NG-RAN Access Point Position) nor the
// provisioned cell-position table provided coordinates for the serving cell.
// Per TS 29.572 a LocationData without a locationEstimate is invalid, so the
// API turns this into a failure response rather than a coordinate-less body.
var ErrNoLocationEstimate = errors.New("no location estimate available for serving cell")

// resolveCellCoordinate returns the geographic anchor for the UE's serving
// cell, preferring a RAN-supplied NG-RAN Access Point Position (E-CID) and
// falling back to the provisioned cell-position table. Returns false when no
// coordinate is available from any source.
func (l *LMF) resolveCellCoordinate(ctx context.Context, loc coremodels.UserLocation, measurements *models.RadioMeasurements) (*models.GeographicEstimate, bool) {
	if measurements != nil && measurements.APPosition != nil {
		return measurements.APPosition, true
	}

	if l.db == nil {
		return nil, false
	}

	rat, mcc, mnc, cellID, ok := servingCellKey(loc)
	if !ok {
		return nil, false
	}

	cp, err := l.db.GetCellPositionByCell(ctx, rat, mcc, mnc, cellID)
	if err != nil {
		return nil, false
	}

	return cellPositionEstimate(cp), true
}

// servingCellKey extracts the (rat, mcc, mnc, cellIdentity) natural key of the
// UE's serving cell from its user-location context.
func servingCellKey(loc coremodels.UserLocation) (rat, mcc, mnc, cellID string, ok bool) {
	if loc.NrLocation != nil && loc.NrLocation.Ncgi.PlmnID != nil {
		return db.RATNR, loc.NrLocation.Ncgi.PlmnID.Mcc, loc.NrLocation.Ncgi.PlmnID.Mnc, loc.NrLocation.Ncgi.NrCellID, true
	}

	if loc.EutraLocation != nil && loc.EutraLocation.Ecgi.PlmnID != nil {
		return db.RATEUTRA, loc.EutraLocation.Ecgi.PlmnID.Mcc, loc.EutraLocation.Ecgi.PlmnID.Mnc, loc.EutraLocation.Ecgi.EutraCellID, true
	}

	return "", "", "", "", false
}

func cellPositionEstimate(cp *db.CellPosition) *models.GeographicEstimate {
	e := &models.GeographicEstimate{
		LatitudeDegrees:  cp.Latitude,
		LongitudeDegrees: cp.Longitude,
		AltitudeMeters:   cp.Altitude,
	}

	switch {
	case cp.UncertaintySemiMajor != nil && cp.UncertaintySemiMinor != nil && cp.OrientationMajor != nil:
		e.UncertaintyEllipse = &models.UncertaintyEllipse{
			SemiMajorMeters:         max(*cp.UncertaintySemiMajor, *cp.UncertaintySemiMinor),
			SemiMinorMeters:         min(*cp.UncertaintySemiMajor, *cp.UncertaintySemiMinor),
			OrientationMajorDegrees: int32(*cp.OrientationMajor),
		}
	case cp.UncertaintySemiMajor != nil && cp.UncertaintySemiMinor != nil:
		radius := max(*cp.UncertaintySemiMajor, *cp.UncertaintySemiMinor)
		e.UncertaintyRadiusMeters = &radius
	case cp.UncertaintySemiMajor != nil:
		e.UncertaintyRadiusMeters = cp.UncertaintySemiMajor
	case cp.UncertaintySemiMinor != nil:
		e.UncertaintyRadiusMeters = cp.UncertaintySemiMinor
	}

	if cp.Confidence != nil {
		confidence := int32(*cp.Confidence)
		e.ConfidencePercent = &confidence
	}

	return e
}
