// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	coremodels "github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// ecidMeasurementTimeout bounds how long determineECIDLocation waits for the
// gNB's NRPPa UEPositioningInformation response before falling back to Cell ID.
// Increased from 3s to 10s to account for gNB processing time and network latency.
const ecidMeasurementTimeout = 10 * time.Second

const ecidUETimeout = 20 * time.Second

// determineECIDLocation computes location using the E-CID (Enhanced Cell ID) method.
// E-CID extends basic Cell ID with radio measurements (RSRP, RSRQ, TA, Rx-Tx)
// to estimate the UE's distance from the gNB. It requires a geographic anchor
// for the serving cell (RAN-supplied NG-RAN Access Point Position or the
// provisioned cell-position table); with no anchor it returns
// ErrNoLocationEstimate, unless E-CID produced radio measurements, which are then
// returned without an estimate. When no radio measurements are available it
// degrades to a Cell-ID estimate (still requiring the anchor).
func (l *LMF) determineECIDLocation(ctx context.Context, supi etsi.SUPI, mode models.PositioningMode) (*models.LocationResult, string, error) {
	loc, ok := l.getUELocation(supi)
	if !ok {
		return nil, "", fmt.Errorf("UE location not available: %w", ErrNotFound)
	}

	if loc.NrLocation == nil && loc.EutraLocation == nil && loc.N3gaLocation == nil {
		return nil, "", fmt.Errorf("no location available for UE: %w", ErrNotFound)
	}

	sessionID, err := l.sessionMgr.CreateSession(ctx, CreateSessionParams{
		SUPI:        supi.String(),
		RequestType: RequestImmediate,
		Method:      RequestedECID,
	})
	if err != nil {
		return nil, "", fmt.Errorf("create positioning session: %w", err)
	}

	result, err := l.runECID(ctx, supi, sessionID, loc, mode)
	if err != nil {
		if failErr := l.sessionMgr.FailSession(ctx, sessionID); failErr != nil {
			logger.LmfLog.Warn("failed to record E-CID session failure", zap.String("session_id", sessionID), zap.Error(failErr))
		}

		return nil, sessionID, err
	}

	if err := l.sessionMgr.CompleteSession(ctx, sessionID, result); err != nil {
		return nil, sessionID, fmt.Errorf("complete positioning session: %w", err)
	}

	return result, sessionID, nil
}

func (l *LMF) runECID(ctx context.Context, supi etsi.SUPI, sessionID string, loc coremodels.UserLocation, mode models.PositioningMode) (*models.LocationResult, error) {
	nr := loc.NrLocation != nil

	method := models.PositioningMethodECID
	if nr {
		method = models.PositioningMethodNRECID
	}

	wantNetwork := mode == "" || mode == models.PositioningModeNetworkBased
	wantUE := mode == "" || mode == models.PositioningModeUEAssisted

	var (
		wg      sync.WaitGroup
		network *models.RadioMeasurements
		ue      []models.CellMeasurement
		ueErr   error
	)

	if wantNetwork {
		wg.Go(func() {
			network = l.fetchECIDMeasurements(ctx, supi)
		})
	}

	if wantUE {
		wg.Go(func() {
			ue, ueErr = l.fetchUEECIDMeasurements(ctx, supi, sessionID, nr, ueECIDTimeout(mode))
		})
	}

	wg.Wait()

	// Anchor the estimate to a geographic coordinate: prefer the RAN-supplied
	// NG-RAN Access Point Position, else the provisioned cell-position table.
	// Without a coordinate there is no valid location estimate.
	coord, anchored := l.resolveCellCoordinate(ctx, loc, network)

	result := computeCellIDLocation(supi, loc)
	result.Estimate = coord
	result.Positioning = ecidPositioning(method, wantNetwork, wantUE, network, ueErr, anchored)

	if ueErr != nil {
		logger.LmfLog.Warn("UE-assisted E-CID measurements unavailable",
			logger.SUPI(supi.String()),
			zap.Error(ueErr),
		)
	}

	if network != nil {
		result.Measurements = append(result.Measurements, withDistance(network.Cells)...)
	}

	result.Measurements = append(result.Measurements, markServing(ue, result.NCGI, result.ECGI)...)

	if !anchored && len(result.Measurements) == 0 {
		return nil, ErrNoLocationEstimate
	}

	if len(result.Measurements) > 0 {
		now := time.Now().UTC()
		result.UeLocationTimestamp = &now
		result.AgeOfLocationInfo = 0
	}

	logger.LmfLog.Info("E-CID location computed",
		logger.SUPI(supi.String()),
		zap.String("access_type", result.AccessType),
		zap.String("mode", string(mode)),
		zap.Int("cells", len(result.Measurements)),
		zap.Bool("anchored_by_ran", network != nil && network.APPosition != nil),
	)

	return result, nil
}

func ecidPositioning(method models.PositioningMethod, wantNetwork, wantUE bool, network *models.RadioMeasurements, ueErr error, anchored bool) []models.PositioningAttempt {
	var attempts []models.PositioningAttempt

	anchoredByRAN := network != nil && network.APPosition != nil
	if anchored && !anchoredByRAN {
		attempts = append(attempts, cellIDAttempt())
	}

	if wantNetwork {
		usage := models.PositioningUsageUnsuccess

		switch {
		case anchoredByRAN:
			usage = models.PositioningUsageResultsUsedToGenerate
		case network != nil && len(network.Cells) > 0:
			usage = models.PositioningUsageResultsNotUsed
		}

		attempts = append(attempts, models.PositioningAttempt{Method: method, Mode: models.PositioningModeNetworkBased, Usage: usage})
	}

	if wantUE {
		usage := models.PositioningUsageResultsNotUsed
		if ueErr != nil {
			usage = models.PositioningUsageUnsuccess
		}

		attempts = append(attempts, models.PositioningAttempt{Method: method, Mode: models.PositioningModeUEAssisted, Usage: usage})
	}

	return attempts
}

func markServing(cells []models.CellMeasurement, ncgi *coremodels.Ncgi, ecgi *coremodels.Ecgi) []models.CellMeasurement {
	for i := range cells {
		c := &cells[i]

		c.Serving = c.Serving || models.SameNcgi(c.NCGI, ncgi) || models.SameEcgi(c.ECGI, ecgi)
	}

	return cells
}

func ueECIDTimeout(mode models.PositioningMode) time.Duration {
	if mode == "" {
		return ecidMeasurementTimeout
	}

	return ecidUETimeout
}

func (l *LMF) fetchUEECIDMeasurements(ctx context.Context, supi etsi.SUPI, sessionID string, nr bool, timeout time.Duration) ([]models.CellMeasurement, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session := lpp.NewSession(supi.String(), sessionID, lpp.MethodECID)
	session.SetNRAccess(nr)

	result, err := l.runLPPSession(ctx, supi, session, false)
	if err != nil {
		return nil, err
	}

	return result.Measurements, nil
}

// Estimate distance from the best available timing measurement. Prefer
// the NR timing advance (TS 38.455 Value Timing Advance NR), then the
// legacy E-UTRA timing advance, then Rx-Tx.
func withDistance(cells []models.CellMeasurement) []models.CellMeasurement {
	for i := range cells {
		c := &cells[i]

		var dist float64

		switch {
		case c.NRTimingAdvance != nil:
			dist = nrTAToDistance(int32(*c.NRTimingAdvance))
		case c.TimingAdvance != nil:
			dist = taToDistance(int32(*c.TimingAdvance))
		case c.UERxTxTimeDiff != nil:
			dist = rxTxToDistance(int32(*c.UERxTxTimeDiff))
		default:
			continue
		}

		c.DistanceMeters = &dist
	}

	return cells
}

// ecidMeasurementClient requests and collects E-CID radio measurements from a
// RAN over a positioning protocol: NRPPa (5G) or LPPa (4G).
type ecidMeasurementClient interface {
	RequestMeasurements(ctx context.Context, supi etsi.SUPI, method string) (int64, error)
	WaitForMeasurements(ctx context.Context, supi etsi.SUPI, measurementID int64, notBefore time.Time) (*models.RadioMeasurements, error)
	CancelMeasurements(ctx context.Context, supi etsi.SUPI, measurementID int64)
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
//
// A request for an idle UE pages it and may still be pending on return; the deferred
// cancel discards it, since paging supervision outlives ecidMeasurementTimeout.
func (l *LMF) fetchECIDMeasurements(ctx context.Context, supi etsi.SUPI) *models.RadioMeasurements {
	detached := context.WithoutCancel(ctx)

	ctx, cancel := context.WithTimeout(detached, ecidMeasurementTimeout)
	defer cancel()

	client := l.measurementClient(supi)

	requestedAt := time.Now()

	var measID int64

	err := retryWhilePaging(ctx, func() error {
		var err error

		measID, err = client.RequestMeasurements(ctx, supi, string(RequestedECID))

		return err
	})
	if err != nil {
		logger.LmfLog.Warn("E-CID measurement request failed; falling back to Cell ID",
			logger.SUPI(supi.String()),
			zap.Error(err),
		)

		return nil
	}

	defer client.CancelMeasurements(detached, supi, measID)

	measurements, err := client.WaitForMeasurements(ctx, supi, measID, requestedAt)
	if err != nil {
		logger.LmfLog.Warn("E-CID measurements unavailable; falling back to Cell ID",
			logger.SUPI(supi.String()),
			zap.Error(err),
		)

		return nil
	}

	return measurements
}

// taToDistance converts the E-UTRA Timing Advance report value (LPPa
// TimingAdvanceType1/2, INTEGER 0..7690) to a one-way distance in metres. Per
// TS 36.133 §10.3.1 Table 10.3.1-1 the value maps to round-trip TADV in Ts at
// 2 Ts resolution up to 4096 Ts and 8 Ts above; one-way distance = c·TADV·Ts/2.
func taToDistance(ta int32) float64 {
	const oneWayMetersPerTs = 299792458.0 / (15000.0 * 2048.0) / 2.0

	var tadvTs float64

	switch {
	case ta <= 2047:
		tadvTs = 2 * float64(ta)
	case ta <= 7689:
		tadvTs = 4096 + 8*float64(ta-2048)
	default:
		tadvTs = 49232
	}

	return oneWayMetersPerTs * tadvTs
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
