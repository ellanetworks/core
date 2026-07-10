// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"math"

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

// cellCoordinate is an absolute antenna/coverage anchor resolved for a serving
// cell: WGS-84 degrees plus a horizontal uncertainty radius in metres.
type cellCoordinate struct {
	latitudeDegrees  float64
	longitudeDegrees float64
	altitudeMeters   *float64
	horizontalAccM   uint32
}

// resolveCellCoordinate returns the geographic anchor for the UE's serving
// cell, preferring a RAN-supplied NG-RAN Access Point Position (E-CID) and
// falling back to the provisioned cell-position table. Returns false when no
// coordinate is available from any source.
func (l *LMF) resolveCellCoordinate(ctx context.Context, loc coremodels.UserLocation, measurements *models.RadioMeasurements) (cellCoordinate, bool) {
	// 1. RAN-supplied antenna position (only available via E-CID measurements).
	if measurements != nil && measurements.APPosition != nil {
		ap := measurements.APPosition

		acc := uncertaintyCodeToMeters(ap.UncertaintySemiMajor)
		if minor := uncertaintyCodeToMeters(ap.UncertaintySemiMinor); minor > acc {
			acc = minor
		}

		alt := float64(ap.Altitude)

		return cellCoordinate{
			latitudeDegrees:  ap.LatitudeDegrees,
			longitudeDegrees: ap.LongitudeDegrees,
			altitudeMeters:   &alt,
			horizontalAccM:   uint32(math.Round(acc)),
		}, true
	}

	// 2. Provisioned cell-position table, keyed on the serving NCGI/ECGI.
	if l.db == nil {
		return cellCoordinate{}, false
	}

	rat, mcc, mnc, cellID, ok := servingCellKey(loc)
	if !ok {
		return cellCoordinate{}, false
	}

	cp, err := l.db.GetCellPositionByCell(ctx, rat, mcc, mnc, cellID)
	if err != nil {
		return cellCoordinate{}, false
	}

	return cellCoordinate{
		latitudeDegrees:  cp.Latitude,
		longitudeDegrees: cp.Longitude,
		altitudeMeters:   cp.Altitude,
		horizontalAccM:   coverageRadiusMeters(cp),
	}, true
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

// coverageRadiusMeters derives a single horizontal-accuracy radius (metres)
// from a provisioned cell position. Uses the larger uncertainty semi-axis when
// present; otherwise 0 (unknown).
func coverageRadiusMeters(cp *db.CellPosition) uint32 {
	var r float64

	if cp.UncertaintySemiMajor != nil {
		r = *cp.UncertaintySemiMajor
	}

	if cp.UncertaintySemiMinor != nil && *cp.UncertaintySemiMinor > r {
		r = *cp.UncertaintySemiMinor
	}

	if r < 0 {
		return 0
	}

	return uint32(math.Round(r))
}

// applyCellCoordinate writes a resolved coordinate onto a location result.
// Latitude/longitude are stored in 1e-7 degrees and altitude in metres, per the
// LocationResult contract.
func applyCellCoordinate(result *models.LocationResult, c cellCoordinate) {
	result.Latitude = int32(math.Round(c.latitudeDegrees * 1e7))
	result.Longitude = int32(math.Round(c.longitudeDegrees * 1e7))

	if c.altitudeMeters != nil {
		result.Altitude = int32(math.Round(*c.altitudeMeters))
	}

	result.HorizontalAccuracy = c.horizontalAccM
}

// uncertaintyCodeToMeters decodes a TS 23.032 uncertainty code (0..127) into
// metres: r = C·((1+x)^k − 1) with C = 10 and x = 0.1.
func uncertaintyCodeToMeters(code int64) float64 {
	if code <= 0 {
		return 0
	}

	return 10.0 * (math.Pow(1.1, float64(code)) - 1.0)
}
