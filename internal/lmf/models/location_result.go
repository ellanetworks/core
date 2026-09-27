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

// LocationResult is the location expressed by the LMF for a given UE.
// For Cell ID results, TAI/NCGI/ECGI are populated.
type LocationResult struct {
	SUPI                string            `json:"supi"`
	Method              PositioningMethod `json:"method"`
	TAI                 *models.Tai       `json:"tai,omitempty"`
	NCGI                *models.Ncgi      `json:"ncgi,omitempty"`
	ECGI                *models.Ecgi      `json:"ecgi,omitempty"`
	AccessType          string            `json:"access_type,omitempty"`
	AgeOfLocationInfo   int32             `json:"age_of_location_information,omitempty"`
	UeLocationTimestamp *time.Time        `json:"ue_location_timestamp,omitempty"`

	Estimate *GeographicEstimate `json:"estimate,omitempty"`

	// E-CID fields (populated when Method == PositioningMethodECID)
	RSRP     *int32   `json:"rsrp,omitempty"`           // dBm × 100 (e.g., -8500 = -85 dBm)
	RSRQ     *int32   `json:"rsrq,omitempty"`           // dB × 100
	TA       *int32   `json:"timing_advance,omitempty"` // slots
	Distance *float64 `json:"distance_m,omitempty"`     // estimated from TA or Rx-Tx

	// NR-specific E-CID measurements (SSB/CSI-RS based, TS 38.305 §8.9)
	SSRSRP  *int32 `json:"ss_rsrp,omitempty"`  // SSB-based RSRP, dBm × 100
	SSRSRQ  *int32 `json:"ss_rsrq,omitempty"`  // SSB-based RSRQ, dB × 100
	CSIRSRP *int32 `json:"csi_rsrp,omitempty"` // CSI-RS-based RSRP, dBm × 100
	CSIRSRQ *int32 `json:"csi_rsrq,omitempty"` // CSI-RS-based RSRQ, dB × 100

	// NR-specific timing/angle measurements (TS 38.455 §9.2.5 extension IEs)
	NRTimingAdvance   *int32   `json:"nr_timing_advance,omitempty"`   // Value Timing Advance NR (0..7690)
	UERxTxTimeDiff    *int32   `json:"ue_rx_tx_time_diff,omitempty"`  // UE Rx-Tx Time Difference (0..61565)
	AoAAzimuthDegrees *float64 `json:"aoa_azimuth_degrees,omitempty"` // UL Angle of Arrival azimuth
	AoAZenithDegrees  *float64 `json:"aoa_zenith_degrees,omitempty"`  // UL Angle of Arrival zenith

	// UserLocation preserves the original UserLocation so callers can inspect
	// the full NR/E-UTRA/N3IWF detail. Populated only internally.
	UserLocation models.UserLocation `json:"-"`
}

// GetUserLocation returns the internal UserLocation that produced this result.
func (r *LocationResult) GetUserLocation() models.UserLocation {
	return r.UserLocation
}
