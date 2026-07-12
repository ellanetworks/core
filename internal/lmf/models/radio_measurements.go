// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

// RadioMeasurements holds radio measurements extracted from a positioning
// protocol E-CID result. The AMF (NRPPa, 5G) and the MME (LPPa, 4G) both cache
// it on the UE context for the LMF's E-CID method. An access populates only the
// quantities its radio technology defines; the rest stay nil.
type RadioMeasurements struct {
	RSRP               *int32 // dBm × 100 (e.g., -8500 = -85 dBm)
	RSRQ               *int32 // dB × 100
	TA                 *int32 // Timing Advance in slots
	RxTxTimeDifference *int32 // INTEGER (0..61565) per TS 38.455

	// NR-specific measurements (SSB/CSI-RS based, TS 38.305 §8.9)
	SSRSRP  *int32 // SSB-based RSRP, dBm × 100
	SSRSRQ  *int32 // SSB-based RSRQ, dB × 100
	CSIRSRP *int32 // CSI-RS-based RSRP, dBm × 100
	CSIRSRQ *int32 // CSI-RS-based RSRQ, dB × 100

	// NR-specific timing/angle measurements (TS 38.455 §9.2.5 extension IEs).
	NRTimingAdvance   *int32   // Value Timing Advance NR (0..7690), TS 38.133 mapping
	AoAAzimuthDegrees *float64 // UL Angle of Arrival azimuth, decimal degrees
	AoAZenithDegrees  *float64 // UL Angle of Arrival zenith, decimal degrees (optional)

	// APPosition is the serving cell's access point position, when the RAN
	// reports it in an E-CID measurement result (optional).
	APPosition *APPosition
}

// APPosition is a decoded access point position (NG-RAN, TS 38.455 §9.2.2; or
// E-UTRAN, TS 36.455 §9.2.1), converted to WGS-84 decimal degrees plus the
// reported uncertainty.
type APPosition struct {
	LatitudeDegrees      float64
	LongitudeDegrees     float64
	Altitude             int64
	UncertaintySemiMajor int64
	UncertaintySemiMinor int64
	Confidence           int64
}
