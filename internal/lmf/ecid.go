// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// ecidMeasurementTimeout bounds how long determineECIDLocation waits for the
// gNB's NRPPa UEPositioningInformation response before falling back to Cell ID.
const ecidMeasurementTimeout = 3 * time.Second

// determineECIDLocation computes location using the E-CID (Enhanced Cell ID) method.
// E-CID extends basic Cell ID with radio measurements (RSRP, RSRQ, TA, Rx-Tx)
// to estimate the UE's distance from the gNB.
func (l *LMF) determineECIDLocation(supi etsi.SUPI) (*models.LocationResult, error) {
	// 1. Get serving cell location (same as Cell ID)
	loc, ok := l.amf.GetUELocation(supi)
	if !ok {
		return nil, fmt.Errorf("UE location not available: %w", ErrNotFound)
	}

	if loc.NrLocation == nil && loc.EutraLocation == nil && loc.N3gaLocation == nil {
		return nil, fmt.Errorf("no location available for UE: %w", ErrNotFound)
	}

	// 2. Request radio measurements from the RAN over NRPPa and wait for the
	//    asynchronous UEPositioningInformation response. On any failure (no
	//    RAN connection, timeout, decode error) E-CID degrades gracefully to
	//    Cell ID with measurements left nil.
	measurements := l.fetchECIDMeasurements(supi)

	// 3. Build E-CID result with cell ID + measurements. The result is only
	//    labelled GADECID when the RAN actually supplied measurements; on
	//    fallback it stays a plain Cell ID result so callers aren't told it is
	//    an E-CID fix that carries no measurements.
	result := computeCellIDLocation(supi, loc)

	if measurements != nil {
		result.Shape = models.GADECID
		result.RSRP = measurements.RSRP
		result.RSRQ = measurements.RSRQ
		result.TA = measurements.TA

		// 4. Estimate distance from Timing Advance or Rx-Tx
		if measurements.TA != nil {
			dist := taToDistance(*measurements.TA)
			result.Distance = &dist
		} else if measurements.RxTxTimeDifference != nil {
			dist := rxTxToDistance(*measurements.RxTxTimeDifference)
			result.Distance = &dist
		}

		// 5. If the RAN reported the serving cell's NG-RANAccessPointPosition,
		//    use it for a more precise ellipsoid-point location.
		if ap := measurements.APPosition; ap != nil {
			applyAccessPointPosition(result, ap)
		}
	}

	logger.LmfLog.Info("E-CID location computed",
		zap.String("supi", supi.String()),
		zap.String("access_type", result.AccessType),
		zap.Any("rsrp", result.RSRP),
		zap.Any("ta", result.TA),
		zap.Any("distance_m", result.Distance),
	)

	return result, nil
}

// applyAccessPointPosition upgrades a Cell-ID/E-CID result to an ellipsoid point
// using the serving cell's NG-RANAccessPointPosition reported over NRPPa. The
// latitude/longitude are stored in 1e-7 degrees (per LocationResult), the
// altitude in metres, and the horizontal accuracy from the uncertainty ellipse.
func applyAccessPointPosition(result *models.LocationResult, ap *amf.APPosition) {
	result.Latitude = int32(ap.LatitudeDegrees * 1e7)
	result.Longitude = int32(ap.LongitudeDegrees * 1e7)
	result.Altitude = int32(ap.Altitude)

	// Use the larger uncertainty semi-axis as the horizontal accuracy estimate.
	acc := ap.UncertaintySemiMajor
	if ap.UncertaintySemiMinor > acc {
		acc = ap.UncertaintySemiMinor
	}

	if acc >= 0 {
		result.HorizontalAccuracy = uint32(acc)
	}
}

// fetchECIDMeasurements triggers an NRPPa measurement request to the RAN and
// waits for the matching UEPositioningInformation response. It returns nil
// (so the caller falls back to Cell ID) whenever the request can't be sent or
// no response arrives within ecidMeasurementTimeout.
func (l *LMF) fetchECIDMeasurements(supi etsi.SUPI) *amf.RadioMeasurements {
	ctx, cancel := context.WithTimeout(context.Background(), ecidMeasurementTimeout)
	defer cancel()

	requestedAt := time.Now()

	measID, err := l.nrppaClient.RequestMeasurements(ctx, supi, string(MethodECID))
	if err != nil {
		logger.LmfLog.Warn("E-CID measurement request failed; falling back to Cell ID",
			zap.String("supi", supi.String()),
			zap.Error(err),
		)

		return nil
	}

	measurements, err := l.nrppaClient.WaitForMeasurements(ctx, supi, measID, requestedAt)
	if err != nil {
		logger.LmfLog.Warn("E-CID measurements unavailable; falling back to Cell ID",
			zap.String("supi", supi.String()),
			zap.Error(err),
		)

		return nil
	}

	return measurements
}

// taToDistance converts Timing Advance slots to distance in meters.
// Per 3GPP TS 38.133, 1 TA unit ≈ 78 meters (based on 0.52 μs × speed of light / 2).
func taToDistance(ta int32) float64 {
	return float64(ta) * 78.0
}

// rxTxToDistance converts UE Rx-Tx time difference to distance in meters.
// Distance = (rxTx × speedOfLight) / 2.
func rxTxToDistance(rxTx int32) float64 {
	const speedOfLightMPerUs = 299.792458
	return float64(rxTx) * speedOfLightMPerUs
}
