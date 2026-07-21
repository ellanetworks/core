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

var ErrNotFound = errors.New("UE not found or not registered")

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
// available for the cell it returns ErrNoLocationEstimate (a Cell-ID estimate
// without coordinates is not a valid TS 29.572 LocationData). N3IWF is an
// exception: it carries no cell identifier, so a coordinate-less result is
// returned for non-3GPP access.
//
// Per TS 23.273 §6.5.1 step 12: if the AMF does not have a valid (fresh)
// location, it invokes the NG-RAN Location Reporting procedure by sending
// LocationReportingControl(Direct) to the RAN. The refresh is fire-and-forget:
// the RAN's LocationReport arrives asynchronously on the dispatch goroutine
// and updates the AMF's cached location, so the current request returns the
// existing location and the next request benefits from the refresh.
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

	// TS 23.273 §6.5.1 step 12: check if the AMF's known location is fresh.
	// If stale, actively refresh via LocationReportingControl(Direct). The
	// refresh is async — the LocationReport arrives on the dispatch goroutine
	// and updates the AMF's cache, so this request proceeds with the current
	// (stale) location; the next request will see the refreshed value.
	if IsLocationStale(loc, maxAge) {
		logger.LmfLog.Info("location stale, triggering async refresh",
			zap.String("supi", supi.String()),
			zap.Int32("maxAge", maxAge),
		)

		if err := l.refreshLocation(ctx, supi); err != nil {
			logger.LmfLog.Warn("location refresh failed, returning current location",
				zap.String("supi", supi.String()),
				zap.Error(err),
			)
		}
	}

	result := computeCellIDLocation(supi, loc)

	// N3IWF has no cell coordinate — skip coordinate resolution.
	if loc.N3gaLocation != nil {
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
// maxAgeSeconds. A location is stale when its age (time since the AMF last
// received a location update via NGAP) exceeds maxAgeSeconds, or when it is
// an N3IWF location (which has no refresh mechanism and is always considered
// valid once received).
func IsLocationStale(loc interface{}, maxAgeSeconds int32) bool {
	switch v := loc.(type) {
	case *coremodels.NrLocation:
		if v == nil {
			return true
		}

		if v.UeLocationTimestamp == nil {
			return true
		}

		return time.Since(*v.UeLocationTimestamp).Seconds() > float64(maxAgeSeconds)
	case *coremodels.EutraLocation:
		if v == nil {
			return true
		}

		if v.UeLocationTimestamp == nil {
			return true
		}

		return time.Since(*v.UeLocationTimestamp).Seconds() > float64(maxAgeSeconds)
	case *coremodels.N3gaLocation:
		// N3IWF has no cell-level refresh mechanism; treat as never stale.
		return false
	case coremodels.UserLocation:
		// Check the nested location types directly.
		if v.NrLocation != nil {
			return IsLocationStale(v.NrLocation, maxAgeSeconds)
		}

		if v.EutraLocation != nil {
			return IsLocationStale(v.EutraLocation, maxAgeSeconds)
		}
		// N3IWF or empty — N3IWF is never stale, empty is always stale.
		return v.N3gaLocation == nil
	default:
		// Empty or unknown location is always stale.
		return true
	}
}

// refreshLocation triggers an active location refresh by asking the AMF to
// send a LocationReportingControl(Direct) to the RAN (TS 23.273 §6.5.1
// "Otherwise" branch — invoke NG-RAN Location Reporting procedure).
func (l *LMF) refreshLocation(ctx context.Context, supi etsi.SUPI) error {
	if l.amf == nil {
		return fmt.Errorf("AMF is not available")
	}

	return l.amf.RefreshLocation(ctx, supi)
}
