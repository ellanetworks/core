// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
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
	loc, ok := l.getUELocation(supi)
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

// ecidMeasurementClient requests and collects E-CID radio measurements from a
// RAN over a positioning protocol: NRPPa (5G) or LPPa (4G).
type ecidMeasurementClient interface {
	RequestMeasurements(ctx context.Context, supi etsi.SUPI, method string) (int64, error)
	WaitForMeasurements(ctx context.Context, supi etsi.SUPI, measurementID int64, notBefore time.Time) (*models.RadioMeasurements, error)
}

// measurementClient selects the positioning protocol by the access that owns the
// UE: NRPPa when the AMF holds it (5G), else LPPa when the MME holds it (4G). It
// consults the sources in the same order as getUELocation so the measurement
// protocol always matches the access the serving cell was resolved from.
func (l *LMF) measurementClient(supi etsi.SUPI) ecidMeasurementClient {
	if l.amf != nil {
		if _, ok := l.amf.GetUELocation(supi); ok {
			return l.nrppaClient
		}
	}

	if l.mme != nil {
		if _, ok := l.mme.GetUELocation(supi); ok {
			return l.lppaClient
		}
	}

	return l.nrppaClient
}

// fetchECIDMeasurements triggers a measurement request to the RAN and waits for
// the matching response. It returns nil (so the caller falls back to Cell ID)
// whenever the request can't be sent or no response arrives within
// ecidMeasurementTimeout.
func (l *LMF) fetchECIDMeasurements(supi etsi.SUPI) *models.RadioMeasurements {
	ctx, cancel := context.WithTimeout(context.Background(), ecidMeasurementTimeout)
	defer cancel()

	client := l.measurementClient(supi)

	requestedAt := time.Now()

	measID, err := client.RequestMeasurements(ctx, supi, string(MethodECID))
	if err != nil {
		logger.LmfLog.Warn("E-CID measurement request failed; falling back to Cell ID",
			zap.String("supi", supi.String()),
			zap.Error(err),
		)

		return nil
	}

	measurements, err := client.WaitForMeasurements(ctx, supi, measID, requestedAt)
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

// NR-TADV report-mapping constants, per TS 38.133 clause 13.5.1 "Report
// mapping" (Table 13.5.1-1). The NRPPa "NR-TADV" IE (TS 38.455, INTEGER
// 0..7690) is a non-linear quantization of TADV (TS 38.215 clause 5.2.7),
// expressed in units of Tc (the NR basic time unit, TS 38.211 clause 4.1):
//
//	Tc = 1 / (480000 * 4096) s ≈ 0.5086 ns
//
// The reporting range 0..3150848 Tc is split into two uniform regions:
//   - reported values 0..2047:    128 Tc/step  (TADV 0..262144 Tc)
//   - reported values 2048..7689: 512 Tc/step  (TADV 262144..3150848 Tc)
//   - reported value  7690:       open-ended clipping bin, TADV >= 3150848 Tc
//
// NOTE (per the TS 38.133 table): "TADV is equal to (gNB Rx-Tx time
// difference) + NTA_offset, where NTA_offset is based on the information
// n-TimingAdvanceOffset as specified in TS 38.331". The LMF has no visibility
// into the cell's RRC-signalled n-TimingAdvanceOffset, so the distance
// computed below is offset by that (cell- and duplex-mode-specific) constant;
// treat it as a coarse estimate rather than a corrected round-trip delay.
const (
	nrTADVBasicTimeUnitSeconds = 1.0 / (480000.0 * 4096.0)              // Tc, TS 38.211 §4.1
	nrTADVFineStepTc           = 128                                    // Tc per reported unit, values 0..2047
	nrTADVFineStepCount        = 2048                                   // number of fine-resolution reported values (0..2047)
	nrTADVCoarseStepTc         = 512                                    // Tc per reported unit, values 2048..7689
	nrTADVCoarseStartTc        = nrTADVFineStepCount * nrTADVFineStepTc // 262144 Tc, start of the coarse region
	nrTADVMaxReportedValue     = 7690                                   // open-ended clipping bin (TADV >= 3150848 Tc)
)

// nrTAToDistance converts an NR-TADV report value (TS 38.455 "NR-TADV" IE) to
// an estimated UE-gNB distance in metres, using the TS 38.133 §13.5.1 report
// mapping. Each reported value represents a quantization bin; the bin
// midpoint is used as the representative TADV value (the final open-ended bin
// uses its lower bound, since it has no upper edge). NR-TADV approximates the
// round-trip time, so the one-way distance is c·(TADV_Tc·Tc)/2.
func nrTAToDistance(tadv int32) float64 {
	const speedOfLight = 299792458.0 // m/s

	if tadv < 0 {
		return 0
	}

	var tadvTc float64

	switch {
	case tadv < nrTADVFineStepCount:
		// Fine-resolution region: bin [128n, 128(n+1)) Tc, midpoint 128n+64.
		tadvTc = float64(tadv)*nrTADVFineStepTc + nrTADVFineStepTc/2
	case tadv < nrTADVMaxReportedValue:
		// Coarse-resolution region: bin starts at 262144 Tc.
		idx := tadv - nrTADVFineStepCount
		tadvTc = nrTADVCoarseStartTc + float64(idx)*nrTADVCoarseStepTc + nrTADVCoarseStepTc/2
	default:
		// Open-ended clipping bin (reported value 7690): TADV >= 3150848 Tc.
		// No upper edge exists, so use the bin's lower bound.
		idx := nrTADVMaxReportedValue - nrTADVFineStepCount
		tadvTc = nrTADVCoarseStartTc + float64(idx)*nrTADVCoarseStepTc
	}

	return tadvTc * nrTADVBasicTimeUnitSeconds * speedOfLight / 2.0
}

// rxTxToDistance converts UE Rx-Tx time difference to distance in meters.
// Distance = (rxTx × speedOfLight) / 2.
func rxTxToDistance(rxTx int32) float64 {
	const speedOfLightMPerUs = 299.792458
	return float64(rxTx) * speedOfLightMPerUs
}
