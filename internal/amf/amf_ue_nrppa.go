// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"time"

	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
)

func (ue *UeContext) SetRadioMeasurements(m *lmfmodels.RadioMeasurements) {
	if m == nil {
		return
	}

	ue.radioMu.Lock()
	defer ue.radioMu.Unlock()

	ue.radioMeasurements = m
}

// GetRadioMeasurements returns a copy of the UE's current radio measurements.
func (ue *UeContext) GetRadioMeasurements() *lmfmodels.RadioMeasurements {
	ue.radioMu.RLock()
	defer ue.radioMu.RUnlock()

	if ue.radioMeasurements == nil {
		return nil
	}
	// Shallow copy so callers can't mutate the stored struct. The inner pointer fields
	// are safe to share: SetRadioMeasurements always replaces the whole struct, never
	// mutating fields in place.
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

func (ue *UeContext) GetNRPPaMessages() []NRPPaMessage {
	ue.nrppaMu.RLock()
	defer ue.nrppaMu.RUnlock()

	result := make([]NRPPaMessage, len(ue.nrppaMessages))
	copy(result, ue.nrppaMessages)

	return result
}
