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
	"go.uber.org/zap"
)

var (
	ErrNotFound       = errors.New("UE not found or not registered")
	ErrRefreshTimeout = errors.New("location refresh timed out")
)

// DetermineLocation computes the current location of the UE identified by
// supi using the configured positioning method. Returns the location result,
// the session ID (if a session was created), and any error.
func (l *LMF) DetermineLocation(ctx context.Context, supi etsi.SUPI, method PositioningMethod) (*models.LocationResult, string, error) {
	switch method {
	case MethodCellID:
		result, err := l.determineCellIDLocation(ctx, supi)
		return result, "", err
	case MethodECID:
		result, err := l.determineECIDLocation(ctx, supi)
		return result, "", err
	case MethodAGNSSAssisted, MethodAGNSSBased:
		return l.determineAGNSSLocation(ctx, supi, method)
	default:
		return nil, "", fmt.Errorf("unsupported positioning method: %s", method)
	}
}

// determineCellIDLocation computes location using the Cell ID method. It maps
// the serving cell to its provisioned geographic position; if no coordinate is
// available for the cell it returns ErrNoLocationEstimate.
func (l *LMF) determineCellIDLocation(ctx context.Context, supi etsi.SUPI) (*models.LocationResult, error) {
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

	maxAge := l.maxLocationAge
	if maxAge <= 0 {
		maxAge = defaultMaxLocationAge
	}

	if IsLocationStale(loc, maxAge) {
		logger.LmfLog.Info("location stale, triggering refresh",
			zap.String("supi", supi.String()),
			zap.Int32("maxAge", maxAge),
		)

		refreshed, err := l.refreshLocation(ctx, supi, loc)
		if err != nil {
			logger.LmfLog.Warn("location refresh failed, returning stale location",
				zap.String("supi", supi.String()),
				zap.Error(err),
			)
		} else {
			loc = refreshed
		}
	}

	result := computeCellIDLocation(supi, loc)

	// N3IWF has no cell coordinate — skip coordinate resolution.
	if loc.N3gaLocation != nil && loc.EutraLocation == nil && loc.NrLocation == nil {
		logger.LmfLog.Info("location computed",
			zap.String("supi", supi.String()),
			zap.String("method", "cell_id"),
			zap.String("access_type", result.AccessType),
		)

		return result, nil
	}

	coord, ok := l.resolveCellCoordinate(ctx, loc, nil)
	if !ok {
		return nil, ErrNoLocationEstimate
	}

	applyCellCoordinate(result, coord)

	logger.LmfLog.Info("location computed",
		zap.String("supi", supi.String()),
		zap.String("method", "cell_id"),
		zap.String("access_type", result.AccessType),
	)

	return result, nil
}

// determineAGNSSLocation computes location using A-GNSS via the LPP state
// machine. For AGNSS-assisted (UE-assisted): LMF requests capabilities, then
// requests location, and extracts the fix from ProvideLocationInformation.
// For AGNSS-based: LMF sends assistance data and waits for the UE to compute.
// Returns the location result, session ID, and any error.
func (l *LMF) determineAGNSSLocation(ctx context.Context, supi etsi.SUPI, method PositioningMethod) (*models.LocationResult, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	logger.LmfLog.Info("A-GNSS positioning via LPP",
		zap.String("supi", supi.String()),
		zap.String("method", string(method)),
	)

	// Create LPP session
	session, err := l.sessionMgr.CreateLPPSession(ctx, CreateSessionParams{
		SUPI:              supi.String(),
		Method:            method,
		QoSResponseTimeMs: nil,
		QOSHAccuracyM:     nil,
	})
	if err != nil {
		return nil, "", fmt.Errorf("create LPP session: %w", err)
	}

	// TS 23.273 §6.11.1: one AMF-assigned LCS correlation identifier is used for
	// every message of the positioning session (NOTE 11). Assign it before the
	// session is registered for uplink routing.
	if l.amf != nil {
		session.SetCorrelationID(l.amf.AllocateLCSCorrelationID())
	}

	// Wire transport functions.
	// These closures are called from a different goroutine (AMF's UL NAS handler)
	// and must not use the timeout ctx which may be cancelled when determineAGNSSLocation returns.
	session.SetTransport(
		func(lppMsg []byte) error {
			return l.lppHandler.ForwardLPPToUE(context.Background(), supi.String(), session.CorrelationID(), lppMsg)
		},
		func(result *models.LocationResult) error {
			return l.sessionMgr.CompleteSession(context.Background(), session.SessionID(), result)
		},
		func() error {
			return l.sessionMgr.FailSession(context.Background(), session.SessionID())
		},
		func() error {
			return l.sessionMgr.CancelSession(context.Background(), session.SessionID())
		},
		func() {
			l.DeregisterLPPSession(session.SessionID())
		},
	)

	// Register with LMF for UL message routing
	l.RegisterLPPSession(session.SessionID(), session)

	// Start the LPP state machine (sends RequestLocationInformation for capabilities)
	if err := session.StartSession(); err != nil {
		session.Fail()
		return nil, session.SessionID(), fmt.Errorf("start LPP session: %w", err)
	}

	// Wait for location fix with timeout
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			session.Fail()
			return nil, session.SessionID(), fmt.Errorf("AGNSS positioning timed out: %w", ctx.Err())
		case <-ticker.C:
			state := session.State()
			if state == lpp.LocationReceived {
				result := session.LocationResult()
				if result == nil {
					return nil, session.SessionID(), fmt.Errorf("location result is nil after LocationReceived")
				}

				return &models.LocationResult{
					SUPI:               supi.String(),
					Shape:              models.GADEllipsoidalPoint,
					Latitude:           result.Latitude,
					Longitude:          result.Longitude,
					Altitude:           result.Altitude,
					HorizontalAccuracy: result.HorizontalAccuracy,
					VerticalAccuracy:   result.VerticalAccuracy,
				}, session.SessionID(), nil
			}

			if state == lpp.SessionFailed {
				return nil, session.SessionID(), fmt.Errorf("LPP session failed (state=%s)", state)
			}
		}
	}
}

// computeCellIDLocation converts an AMF UserLocation into a LocationResult
// using the Cell ID positioning method.
func computeCellIDLocation(supi etsi.SUPI, loc coremodels.UserLocation) *models.LocationResult {
	if loc.NrLocation != nil {
		return &models.LocationResult{
			SUPI:                supi.String(),
			Shape:               models.GADCellID,
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
			Shape:               models.GADCellID,
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
			Shape:        models.GADCellID,
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
	timeout := defaultRefreshTimeout
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
