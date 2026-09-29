// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"time"

	"github.com/ellanetworks/core/internal/models"
)

type PositioningMethod int

const (
	PositioningMethodCellID PositioningMethod = iota
	PositioningMethodGNSS
	PositioningMethodECID
	PositioningMethodNRECID
)

type PositioningMode string

const (
	PositioningModeUEBased      PositioningMode = "ue_based"
	PositioningModeUEAssisted   PositioningMode = "ue_assisted"
	PositioningModeStandalone   PositioningMode = "standalone"
	PositioningModeNetworkBased PositioningMode = "network_based"
)

type PositioningUsage string

const (
	PositioningUsageUnsuccess             PositioningUsage = "UNSUCCESS"
	PositioningUsageResultsNotUsed        PositioningUsage = "SUCCESS_RESULTS_NOT_USED"
	PositioningUsageResultsUsedToGenerate PositioningUsage = "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION"
)

type PositioningAttempt struct {
	Method PositioningMethod `json:"method"`
	Mode   PositioningMode   `json:"mode"`
	Usage  PositioningUsage  `json:"usage"`
}

// LocationResult is the location expressed by the LMF for a given UE.
// For Cell ID results, TAI/NCGI/ECGI are populated.
type LocationResult struct {
	SUPI                string       `json:"supi"`
	TAI                 *models.Tai  `json:"tai,omitempty"`
	NCGI                *models.Ncgi `json:"ncgi,omitempty"`
	ECGI                *models.Ecgi `json:"ecgi,omitempty"`
	AccessType          string       `json:"access_type,omitempty"`
	AgeOfLocationInfo   int32        `json:"age_of_location_information,omitempty"`
	UeLocationTimestamp *time.Time   `json:"ue_location_timestamp,omitempty"`

	Estimate *GeographicEstimate `json:"estimate,omitempty"`

	Positioning  []PositioningAttempt `json:"positioning,omitempty"`
	Measurements []CellMeasurement    `json:"measurements,omitempty"`

	// UserLocation preserves the original UserLocation so callers can inspect
	// the full NR/E-UTRA/N3IWF detail. Populated only internally.
	UserLocation models.UserLocation `json:"-"`
}

// GetUserLocation returns the internal UserLocation that produced this result.
func (r *LocationResult) GetUserLocation() models.UserLocation {
	return r.UserLocation
}
