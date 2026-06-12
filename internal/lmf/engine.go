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
func (l *LMF) DetermineLocation(supi etsi.SUPI, method PositioningMethod) (*models.LocationResult, string, error) {
	switch method {
	case MethodCellID:
		result, err := l.determineCellIDLocation(supi)
		return result, "", err
	case MethodECID:
		result, err := l.determineECIDLocation(supi)
		return result, "", err
	case MethodAGNSSAssisted, MethodAGNSSBased:
		return l.determineAGNSSLocation(supi, method)
	default:
		return nil, "", fmt.Errorf("unsupported positioning method: %s", method)
	}
}

// determineCellIDLocation computes location using the Cell ID method.
func (l *LMF) determineCellIDLocation(supi etsi.SUPI) (*models.LocationResult, error) {
	if !l.amf.IsUERegistered(supi) {
		return nil, fmt.Errorf("UE not registered: %w", ErrNotFound)
	}

	loc, ok := l.amf.GetUELocation(supi)
	if !ok {
		return nil, fmt.Errorf("UE location not available: %w", ErrNotFound)
	}

	if loc.NrLocation == nil && loc.EutraLocation == nil && loc.N3gaLocation == nil {
		return nil, fmt.Errorf("no location available for UE: %w", ErrNotFound)
	}

	result := computeCellIDLocation(supi, loc)

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
func (l *LMF) determineAGNSSLocation(supi etsi.SUPI, method PositioningMethod) (*models.LocationResult, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	// Wire transport functions
	session.SetTransport(
		func(lppMsg []byte) error {
			return l.lppHandler.ForwardLPPToUE(ctx, supi.String(), lppMsg)
		},
		func(result *models.LocationResult) error {
			return l.sessionMgr.CompleteSession(ctx, session.SessionID(), result)
		},
		func() error {
			return l.sessionMgr.FailSession(ctx, session.SessionID())
		},
		func() error {
			return l.sessionMgr.CancelSession(ctx, session.SessionID())
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
		}
	}

	if loc.N3gaLocation != nil {
		return &models.LocationResult{
			SUPI:       supi.String(),
			Shape:      models.GADCellID,
			TAI:        loc.N3gaLocation.N3gppTai,
			AccessType: "N3IWF",
		}
	}

	return nil
}
