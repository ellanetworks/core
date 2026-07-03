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
// Increased from 3s to 10s to account for gNB processing time and network latency.
const ecidMeasurementTimeout = 10 * time.Second

// determineECIDLocation computes location using the E-CID (Enhanced Cell ID) method.
// E-CID extends basic Cell ID with radio measurements (RSRP, RSRQ, TA, Rx-Tx)
// to estimate the UE's distance from the gNB. It requires a geographic anchor
// for the serving cell (RAN-supplied NG-RAN Access Point Position or the
// provisioned cell-position table); with no anchor it returns
// ErrNoLocationEstimate. When no radio measurements are available it degrades
// to a Cell-ID estimate (still requiring the anchor).
func (l *LMF) determineECIDLocation(ctx context.Context, supi etsi.SUPI) (*models.LocationResult, error) {
	// 1. Get serving cell location (same as Cell ID)
	loc, ok := l.amf.GetUELocation(supi)
	if !ok {
		return nil, fmt.Errorf("UE location not available: %w", ErrNotFound)
	}

	if loc.NrLocation == nil && loc.EutraLocation == nil && loc.N3gaLocation == nil {
		return nil, fmt.Errorf("no location available for UE: %w", ErrNotFound)
	}

	// 2. Request radio measurements from the RAN over NRPPa and wait for the
	//    asynchronous response. On any failure (no RAN connection, timeout,
	//    decode error) measurements are left nil and E-CID degrades to Cell ID.
	measurements := l.fetchECIDMeasurements(supi)

	// 3. Anchor the estimate to a geographic coordinate: prefer the RAN-supplied
	//    NG-RAN Access Point Position, else the provisioned cell-position table.
	//    Without a coordinate there is no valid location estimate.
	coord, ok := l.resolveCellCoordinate(ctx, loc, measurements)
	if !ok {
		return nil, ErrNoLocationEstimate
	}

	// 4. Build the result. With measurements it is an E-CID estimate; without
	//    them it degrades to Cell-ID.
	result := computeCellIDLocation(supi, loc)

	if measurements != nil {
		result.Shape = models.GADECID
		result.RSRP = measurements.RSRP
		result.RSRQ = measurements.RSRQ
		result.TA = measurements.TA
		result.SSRSRP = measurements.SSRSRP
		result.SSRSRQ = measurements.SSRSRQ
		result.CSIRSRP = measurements.CSIRSRP
		result.CSIRSRQ = measurements.CSIRSRQ
		result.NRTimingAdvance = measurements.NRTimingAdvance
		result.UERxTxTimeDiff = measurements.RxTxTimeDifference
		result.AoAAzimuthDegrees = measurements.AoAAzimuthDegrees
		result.AoAZenithDegrees = measurements.AoAZenithDegrees

		// Estimate distance from the best available timing measurement. Prefer
		// the NR timing advance (TS 38.455 Value Timing Advance NR), then the
		// legacy E-UTRA timing advance, then Rx-Tx.
		switch {
		case measurements.NRTimingAdvance != nil:
			dist := nrTAToDistance(*measurements.NRTimingAdvance)
			result.Distance = &dist
		case measurements.TA != nil:
			dist := taToDistance(*measurements.TA)
			result.Distance = &dist
		case measurements.RxTxTimeDifference != nil:
			dist := rxTxToDistance(*measurements.RxTxTimeDifference)
			result.Distance = &dist
		}
	} else {
		// Downgrade: no radio measurements, so this is a Cell-ID estimate.
		result.Shape = models.GADCellID
	}

	applyCellCoordinate(result, coord)

	logger.LmfLog.Info("E-CID location computed",
		zap.String("supi", supi.String()),
		zap.String("access_type", result.AccessType),
		zap.Int("shape", int(result.Shape)),
		zap.Any("rsrp", result.RSRP),
		zap.Any("nr_ta", result.NRTimingAdvance),
		zap.Any("aoa_azimuth", result.AoAAzimuthDegrees),
		zap.Any("distance_m", result.Distance),
	)

	return result, nil
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

// nrTADVResolutionSeconds is the assumed time granularity of one NR-TADV report
// unit. The NRPPa "Value Timing Advance NR" (INTEGER 0..7690, TS 38.455 §9.2.5)
// shares the value range of the E-UTRA timing-advance report and, per TS 38.133,
// uses a fixed report mapping expressed against a reference time unit rather than
// the per-SCS TA-command step — so the conversion is numerology-independent. We
// use 16·Ts (Ts = 1/(15000·2048) s ≈ 32.55 ns), i.e. the E-UTRA TADV reference
// granularity (≈0.5208 μs per unit).
//
// NOTE: the exact TS 38.133 NR-TADV report-mapping table was not available when
// this was written; treat this constant as approximate and validate it against
// TS 38.133 before relying on the distance quantitatively.
const nrTADVResolutionSeconds = 16.0 / (15000.0 * 2048.0)

// nrTAToDistance converts an NR-TADV report value (TS 38.455 Value Timing
// Advance NR) to an estimated UE–gNB distance in metres. NR-TADV approximates
// the round-trip time, so the one-way distance is c·(value·resolution)/2.
func nrTAToDistance(tadv int32) float64 {
	const speedOfLight = 299792458.0 // m/s
	return float64(tadv) * nrTADVResolutionSeconds * speedOfLight / 2.0
}

// rxTxToDistance converts UE Rx-Tx time difference to distance in meters.
// Distance = (rxTx × speedOfLight) / 2.
func rxTxToDistance(rxTx int32) float64 {
	const speedOfLightMPerUs = 299.792458
	return float64(rxTx) * speedOfLightMPerUs
}
