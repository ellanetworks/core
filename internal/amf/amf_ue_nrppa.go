// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"time"
)

// RadioMeasurements holds radio measurements extracted from NGAP LocationReport
// or NRPPa messages. Used by E-CID positioning to estimate UE distance from gNB.
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

	// APPosition is the serving cell's NG-RANAccessPointPosition, when the RAN
	// reports it in an NRPPa E-CID measurement result (optional).
	APPosition *APPosition
}

// APPosition is a decoded NG-RANAccessPointPosition (TS 38.455 §9.2.2),
// converted to WGS-84 decimal degrees plus the reported uncertainty.
type APPosition struct {
	LatitudeDegrees      float64
	LongitudeDegrees     float64
	Altitude             int64
	UncertaintySemiMajor int64
	UncertaintySemiMinor int64
	Confidence           int64
}

// SetRadioMeasurements updates the UE's radio measurements.
func (ue *UeContext) SetRadioMeasurements(m *RadioMeasurements) {
	if m == nil {
		return
	}

	ue.radioMu.Lock()
	defer ue.radioMu.Unlock()

	ue.radioMeasurements = m
}

// GetRadioMeasurements returns a copy of the UE's current radio measurements.
func (ue *UeContext) GetRadioMeasurements() *RadioMeasurements {
	ue.radioMu.RLock()
	defer ue.radioMu.RUnlock()

	if ue.radioMeasurements == nil {
		return nil
	}
	// Return a shallow copy so callers can't mutate the stored struct.
	// The inner pointer fields are treated as immutable: SetRadioMeasurements
	// always replaces the whole struct rather than mutating fields in place.
	cp := *ue.radioMeasurements

	return &cp
}

// NRPPaMessage holds a raw NRPPa PDU received from the RAN. The PDU is an
// opaque octet string carried over NGAP UE-associated transport; it is decoded
// by the LMF (internal/nrppa). Correlation is by the LMF-UE-Measurement-ID
// carried inside the decoded PDU, not by any AMF-side field.
type NRPPaMessage struct {
	Payload   []byte
	Timestamp time.Time
}

// SetNRPPaMessage stores a raw NRPPa PDU in the UE's message buffer.
func (ue *UeContext) SetNRPPaMessage(data []byte) {
	ue.nrppaMu.Lock()
	defer ue.nrppaMu.Unlock()

	payload := make([]byte, len(data))
	copy(payload, data)

	msg := NRPPaMessage{
		Payload:   payload,
		Timestamp: time.Now(),
	}

	// Ring buffer: keep last 16 messages
	ue.nrppaMessages = append(ue.nrppaMessages, msg)
	if len(ue.nrppaMessages) > 16 {
		ue.nrppaMessages = ue.nrppaMessages[len(ue.nrppaMessages)-16:]
	}
}

// GetNRPPaMessages returns the recent NRPPa messages for this UE.
func (ue *UeContext) GetNRPPaMessages() []NRPPaMessage {
	ue.nrppaMu.RLock()
	defer ue.nrppaMu.RUnlock()

	result := make([]NRPPaMessage, len(ue.nrppaMessages))
	copy(result, ue.nrppaMessages)

	return result
}
