// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/internal/lmf/models"
)

// RequestedMethod identifies the positioning algorithm to use.
type RequestedMethod string

const (
	RequestedCellID RequestedMethod = "cell_id"
	RequestedECID   RequestedMethod = "ecid"
	RequestedGNSS   RequestedMethod = "gnss"
)

var ErrUnsupportedMethod = errors.New("unsupported positioning method")

var ErrUnsupportedMode = errors.New("unsupported positioning mode")

var supportedModes = map[RequestedMethod][]models.PositioningMode{
	RequestedCellID: nil,
	RequestedECID:   {models.PositioningModeUEAssisted, models.PositioningModeNetworkBased},
	RequestedGNSS:   {models.PositioningModeStandalone},
}

func SupportedModes(method RequestedMethod) []models.PositioningMode {
	return supportedModes[method]
}

func ValidateMode(method RequestedMethod, mode models.PositioningMode) error {
	if mode == "" || slices.Contains(supportedModes[method], mode) {
		return nil
	}

	return fmt.Errorf("%w: %q for method %q", ErrUnsupportedMode, mode, method)
}

var ErrMeasurementQuantitiesNotApplicable = errors.New("measurement quantities only apply to network-based E-CID")

type LocationRequest struct {
	Method                RequestedMethod
	Mode                  models.PositioningMode
	MeasurementQuantities *models.MeasurementQuantities
}

func (r LocationRequest) Validate() error {
	if _, ok := supportedModes[r.Method]; !ok {
		return fmt.Errorf("%w: %q", ErrUnsupportedMethod, r.Method)
	}

	if err := ValidateMode(r.Method, r.Mode); err != nil {
		return err
	}

	if r.MeasurementQuantities == nil {
		return nil
	}

	if r.Method != RequestedECID || r.Mode == models.PositioningModeUEAssisted {
		return fmt.Errorf("%w: method %q, mode %q", ErrMeasurementQuantitiesNotApplicable, r.Method, r.Mode)
	}

	return r.MeasurementQuantities.Validate()
}
