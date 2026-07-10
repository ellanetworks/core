// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
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
	default:
		return nil, "", fmt.Errorf("unsupported positioning method: %s", method)
	}
}

// determineCellIDLocation computes location using the Cell ID method. It maps
// the serving cell to its provisioned geographic position; if no coordinate is
// available for the cell it returns ErrNoLocationEstimate (a Cell-ID estimate
// without coordinates is not a valid TS 29.572 LocationData).
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

	result := computeCellIDLocation(supi, loc)

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
