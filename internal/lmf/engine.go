// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	coremodels "github.com/ellanetworks/core/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrNotFound       = errors.New("UE not found or not registered")
	ErrRefreshTimeout = errors.New("location refresh timed out")
)

func (l *LMF) DetermineLocation(ctx context.Context, supi etsi.SUPI, req LocationRequest) (*models.LocationResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var result *models.LocationResult

	err := l.withUE(ctx, supi, func() error {
		var err error

		result, err = l.locate(ctx, supi, req, l.locationMaxAge())

		return err
	})

	return result, err
}

func (l *LMF) locate(ctx context.Context, supi etsi.SUPI, req LocationRequest, maxAge int32) (*models.LocationResult, error) {
	switch req.Method {
	case RequestedCellID:
		return l.determineCellIDLocation(ctx, supi, maxAge)
	case RequestedECID:
		return l.determineECIDLocation(ctx, supi, req.Mode, req.MeasurementQuantities)
	case RequestedGNSS:
		return l.determineGNSSLocation(ctx, supi)
	default:
		return nil, fmt.Errorf("unsupported positioning method: %s", req.Method)
	}
}

func (l *LMF) locationMaxAge() int32 {
	if l.maxLocationAge <= 0 {
		return defaultMaxLocationAge
	}

	return l.maxLocationAge
}

func (l *LMF) determineCellIDLocation(ctx context.Context, supi etsi.SUPI, maxAge int32) (*models.LocationResult, error) {
	if !l.isUERegistered(supi) {
		return nil, fmt.Errorf("UE not registered: %w", ErrNotFound)
	}

	loc, ok := l.getUELocation(supi)
	if !ok {
		return nil, fmt.Errorf("UE location not available: %w", ErrNotFound)
	}

	if loc.NrLocation == nil && loc.EutraLocation == nil && loc.N3gaLocation == nil {
		return nil, fmt.Errorf("no location available for UE: %w", ErrNotFound)
	}

	if IsLocationStale(loc, maxAge) {
		logger.LmfLog.Info("location stale, triggering refresh",
			logger.SUPI(supi.String()),
			zap.Int32("max_age", maxAge),
		)

		refreshed, err := l.refreshLocation(ctx, supi, loc)
		if err != nil {
			logger.LmfLog.Warn("location refresh failed, returning stale location",
				logger.SUPI(supi.String()),
				zap.Error(err),
			)
		} else {
			loc = refreshed
		}
	}

	result := computeCellIDLocation(supi, loc)

	coord, ok := l.resolveCellCoordinate(ctx, loc, nil)
	if !ok {
		return nil, ErrNoLocationEstimate
	}

	result.Estimate = coord
	result.Positioning = []models.PositioningAttempt{cellIDAttempt()}

	logger.LmfLog.Info("location computed",
		logger.SUPI(supi.String()),
		zap.String("method", "cell_id"),
		zap.String("access_type", result.AccessType),
	)

	return result, nil
}

func (l *LMF) determineGNSSLocation(ctx context.Context, supi etsi.SUPI) (*models.LocationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	logger.LmfLog.Info("GNSS positioning via LPP", logger.SUPI(supi.String()))

	session := lpp.NewSession(supi.String(), uuid.NewString(), string(RequestedGNSS))

	result, err := l.runLPPSession(ctx, supi, session)
	if err != nil {
		return nil, fmt.Errorf("GNSS positioning: %w", err)
	}

	return result, nil
}

func (l *LMF) runLPPSession(ctx context.Context, supi etsi.SUPI, session *lpp.Session) (*models.LocationResult, error) {
	// TS 23.273 §6.11.1: one AMF- or MME-assigned LCS correlation identifier is used for
	// every message of the positioning session (NOTE 11). Assign it before the
	// session is registered for uplink routing.
	core := l.servingCore(supi)

	if id := l.allocateLCSCorrelationID(core); id != nil {
		session.SetCorrelationID(id)
	}

	// Wire transport functions.
	// These closures are called from a different goroutine (AMF's UL NAS handler)
	// and must not use the timeout ctx which may be cancelled when the procedure returns.
	detached := context.WithoutCancel(ctx)

	session.SetTransport(
		func(lppMsg []byte) error {
			return retryWhilePaging(ctx, func() error {
				return l.lppHandler.ForwardLPPToUE(detached, core, supi.String(), session.CorrelationID(), lppMsg)
			})
		},
		func() {
			l.DeregisterLPPSession(session.SessionID())
		},
	)

	if deadline, ok := ctx.Deadline(); ok {
		session.SetDeadline(deadline)
	}

	// Register with LMF for UL message routing
	l.RegisterLPPSession(session.SessionID(), session)

	// Start the LPP state machine (sends RequestLocationInformation for capabilities)
	if err := session.StartSession(); err != nil {
		session.FailWith(err)
		return nil, fmt.Errorf("start LPP session: %w", err)
	}

	// Wait for location fix with timeout
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			session.Fail()

			// Discard any buffered LPP: this session has failed.
			l.cancelBufferedLPP(detached, core, supi, session.CorrelationID())

			return nil, fmt.Errorf("LPP session timed out: %w", ctx.Err())
		case <-ticker.C:
			state := session.State()
			if state == lpp.LocationReceived {
				result := session.LocationResult()
				if result == nil {
					return nil, fmt.Errorf("location result is nil after LocationReceived")
				}

				return result, nil
			}

			if state == lpp.SessionFailed {
				if failure := session.Failure(); failure != nil {
					return nil, fmt.Errorf("LPP session failed: %w", failure)
				}

				return nil, fmt.Errorf("LPP session failed (state=%s)", state)
			}
		}
	}
}

func cellIDAttempt() models.PositioningAttempt {
	return models.PositioningAttempt{
		Method: models.PositioningMethodCellID,
		Mode:   models.PositioningModeNetworkBased,
		Usage:  models.PositioningUsageResultsUsedToGenerate,
	}
}

// computeCellIDLocation converts an AMF UserLocation into a LocationResult
// using the Cell ID positioning method.
func computeCellIDLocation(supi etsi.SUPI, loc coremodels.UserLocation) *models.LocationResult {
	if loc.NrLocation != nil {
		return &models.LocationResult{
			SUPI:                supi.String(),
			TAI:                 loc.NrLocation.Tai,
			NCGI:                loc.NrLocation.Ncgi,
			AccessType:          "NR",
			AgeOfLocationInfo:   loc.NrLocation.AgeOfLocationInformation,
			UeLocationTimestamp: loc.NrLocation.UeLocationTimestamp,
			UserLocation:        loc,
		}
	}

	if loc.EutraLocation != nil {
		return &models.LocationResult{
			SUPI:                supi.String(),
			TAI:                 loc.EutraLocation.Tai,
			ECGI:                loc.EutraLocation.Ecgi,
			AccessType:          "EUTRA",
			AgeOfLocationInfo:   loc.EutraLocation.AgeOfLocationInformation,
			UeLocationTimestamp: loc.EutraLocation.UeLocationTimestamp,
			UserLocation:        loc,
		}
	}

	if loc.N3gaLocation != nil {
		return &models.LocationResult{
			SUPI:         supi.String(),
			TAI:          loc.N3gaLocation.N3gppTai,
			AccessType:   "N3IWF",
			UserLocation: loc,
		}
	}

	return nil
}

// IsLocationStale reports whether loc is considered stale relative to
// maxAgeSeconds.
func IsLocationStale(loc coremodels.UserLocation, maxAgeSeconds int32) bool {
	if loc.NrLocation != nil {
		if loc.NrLocation.UeLocationTimestamp == nil {
			return true
		}

		return time.Since(*loc.NrLocation.UeLocationTimestamp).Seconds() > float64(maxAgeSeconds)
	}

	if loc.EutraLocation != nil {
		if loc.EutraLocation.UeLocationTimestamp == nil {
			return true
		}

		return time.Since(*loc.EutraLocation.UeLocationTimestamp).Seconds() > float64(maxAgeSeconds)
	}

	// N3IWF or empty — N3IWF is never stale, empty is always stale.
	return loc.N3gaLocation == nil
}

// refreshLocation triggers an active location refresh through the AMF via the
// NG-RAN location reporting procedure (TS 23.273 §6.5.1 step 12). It waits for
// the AMF to receive a LocationReport from the RAN, polling the AMF's location
// cache until the timestamp changes or the timeout expires.
//
// Returns the fresh location on success, or the stale location wrapped with
// ErrRefreshTimeout if the refresh does not complete in time.
func (l *LMF) refreshLocation(ctx context.Context, supi etsi.SUPI, loc coremodels.UserLocation) (coremodels.UserLocation, error) {
	if l.amf == nil {
		return coremodels.UserLocation{}, fmt.Errorf("AMF is not available")
	}

	ue, ok := l.amf.LookupUeBySupi(supi)
	if !ok {
		return coremodels.UserLocation{}, fmt.Errorf("UE not found: %w", ErrNotFound)
	}

	// Record the current timestamp before triggering the refresh.
	staleTime := time.Time{}
	if loc.NrLocation != nil && loc.NrLocation.UeLocationTimestamp != nil {
		staleTime = *loc.NrLocation.UeLocationTimestamp
	} else if loc.EutraLocation != nil && loc.EutraLocation.UeLocationTimestamp != nil {
		staleTime = *loc.EutraLocation.UeLocationTimestamp
	}

	// Trigger the refresh (sends LocationReportingControl(Direct) to RAN).
	if err := l.amf.RefreshLocation(ctx, supi); err != nil {
		return coremodels.UserLocation{}, fmt.Errorf("send refresh request: %w", err)
	}

	// Wait for the RAN to respond with a LocationReport that updates the timestamp.
	timeout := l.refreshTimeout
	if timeout <= 0 {
		timeout = defaultRefreshTimeout
	}

	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d < timeout {
			timeout = d
		}
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return coremodels.UserLocation{}, fmt.Errorf("context cancelled: %w", ErrRefreshTimeout)
		case <-deadline:
			return coremodels.UserLocation{}, fmt.Errorf("refresh did not complete in %v: %w", timeout, ErrRefreshTimeout)
		case <-ticker.C:
			loc := ue.GetUserLocation()

			var freshTime time.Time
			if loc.NrLocation != nil && loc.NrLocation.UeLocationTimestamp != nil {
				freshTime = *loc.NrLocation.UeLocationTimestamp
			} else if loc.EutraLocation != nil && loc.EutraLocation.UeLocationTimestamp != nil {
				freshTime = *loc.EutraLocation.UeLocationTimestamp
			}

			if !freshTime.IsZero() && !freshTime.Equal(staleTime) {
				return loc, nil
			}
		}
	}
}
