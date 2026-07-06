// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"time"

	"github.com/ellanetworks/core/internal/models"
)

// GADShape identifies the geometric area descriptor shape used to express
// a location result (per TS 23.032).
type GADShape int

const (
	GADCellID GADShape = iota
	GADEllipsoidalPoint
	GADCircle
	GADPolygon
	GADECID
)

// LocationResult is the location expressed by the LMF for a given UE.
// For Cell ID results, TAI/NCGI/ECGI are populated.
// For GNSS results, Latitude/Longitude/Altitude are populated (in 1e-7 deg / cm).
type LocationResult struct {
	SUPI                string       `json:"supi"`
	Shape               GADShape     `json:"shape"`
	TAI                 *models.Tai  `json:"tai,omitempty"`
	NCGI                *models.Ncgi `json:"ncgi,omitempty"`
	ECGI                *models.Ecgi `json:"ecgi,omitempty"`
	AccessType          string       `json:"access_type,omitempty"`
	AgeOfLocationInfo   int32        `json:"age_of_location_information,omitempty"`
	UeLocationTimestamp *time.Time   `json:"ue_location_timestamp,omitempty"`

	// GNSS fields (populated when Shape == GADEllipsoidalPoint, etc.)
	Latitude           int32  `json:"latitude,omitempty"`            // in 1e-7 degrees
	Longitude          int32  `json:"longitude,omitempty"`           // in 1e-7 degrees
	Altitude           int32  `json:"altitude,omitempty"`            // in cm (WGS84 ellipsoid)
	HorizontalAccuracy uint32 `json:"horizontal_accuracy,omitempty"` // in meters
	VerticalAccuracy   uint32 `json:"vertical_accuracy,omitempty"`   // in meters

	// E-CID fields (populated when Shape == GADECID)
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
}
