// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "github.com/ellanetworks/core/internal/models"

type RAT string

const (
	RATNR    RAT = "NR"
	RATEUTRA RAT = "EUTRA"
)

type MeasurementSource string

const (
	MeasurementSourceUE      MeasurementSource = "UE"
	MeasurementSourceNetwork MeasurementSource = "NETWORK"
)

type ARFCNType string

const (
	ARFCNTypeSSB         ARFCNType = "SSB"
	ARFCNTypeCSIRSPointA ARFCNType = "CSI_RS_POINT_A"
)

type BeamMeasurement struct {
	Index int64    `json:"index"`
	RSRP  *float64 `json:"rsrp,omitempty"`
	RSRQ  *float64 `json:"rsrq,omitempty"`
}

type CellMeasurement struct {
	Source            MeasurementSource `json:"source"`
	RAT               RAT               `json:"rat"`
	Serving           bool              `json:"serving"`
	PCI               *int64            `json:"pci,omitempty"`
	ARFCN             *int64            `json:"arfcn,omitempty"`
	ARFCNType         ARFCNType         `json:"arfcn_type,omitempty"`
	NCGI              *models.Ncgi      `json:"ncgi,omitempty"`
	ECGI              *models.Ecgi      `json:"ecgi,omitempty"`
	RSRP              *float64          `json:"rsrp,omitempty"`
	RSRQ              *float64          `json:"rsrq,omitempty"`
	SSRSRP            *float64          `json:"ss_rsrp,omitempty"`
	SSRSRQ            *float64          `json:"ss_rsrq,omitempty"`
	CSIRSRP           *float64          `json:"csi_rsrp,omitempty"`
	CSIRSRQ           *float64          `json:"csi_rsrq,omitempty"`
	SSBBeams          []BeamMeasurement `json:"ssb_beams,omitempty"`
	CSIRSBeams        []BeamMeasurement `json:"csi_rs_beams,omitempty"`
	TimingAdvance     *int64            `json:"timing_advance,omitempty"`
	NRTimingAdvance   *int64            `json:"nr_timing_advance,omitempty"`
	UERxTxTimeDiff    *int64            `json:"ue_rx_tx_time_diff,omitempty"`
	AoAAzimuthDegrees *float64          `json:"aoa_azimuth_degrees,omitempty"`
	AoAZenithDegrees  *float64          `json:"aoa_zenith_degrees,omitempty"`
	DistanceMeters    *float64          `json:"distance_m,omitempty"`
}

// RadioMeasurements holds radio measurements extracted from a positioning
// protocol E-CID result. The AMF (NRPPa, 5G) and the MME (LPPa, 4G) both cache
// it on the UE context for the LMF's E-CID method. An access populates only the
// quantities its radio technology defines; the rest stay nil.
type RadioMeasurements struct {
	Cells []CellMeasurement

	// APPosition is the serving cell's access point position, when the RAN
	// reports it in an E-CID measurement result (optional).
	APPosition *GeographicEstimate
}

func EUTRARSRPDBm(v int64) float64 {
	return float64(v - 141)
}

func EUTRARSRQDB(v int64) float64 {
	return -20 + 0.5*float64(v)
}

func EUTRARSRPExtendedDBm(v int64) float64 {
	if v <= -17 {
		return -157
	}

	return float64(v - 140)
}

func EUTRARSRQExtendedDB(v int64) float64 {
	switch {
	case v < 0:
		return -19.5 + 0.5*float64(v)
	case v <= 34:
		return EUTRARSRQDB(v)
	default:
		return -20.5 + 0.5*float64(v)
	}
}

func NRRSRPDBm(v int64) *float64 {
	if v >= 127 {
		return nil
	}

	dbm := float64(v - 157)

	return &dbm
}

func NRRSRQDB(v int64) float64 {
	return -43 + 0.5*float64(v-1)
}
