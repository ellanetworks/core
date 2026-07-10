// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

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

// LPPaMessage holds a raw LPPa PDU received from the eNB. The PDU is an opaque
// octet string carried over S1AP UE-associated transport; it is decoded by the
// LMF (github.com/ellanetworks/core/lppa). Correlation is by the
// E-SMLC-UE-Measurement-ID carried inside the decoded PDU, not by any MME-side
// field.
type LPPaMessage struct {
	Payload   []byte
	Timestamp time.Time
}

func (ue *UeContext) SetLPPaMessage(data []byte) {
	ue.lppaMu.Lock()
	defer ue.lppaMu.Unlock()

	payload := make([]byte, len(data))
	copy(payload, data)

	msg := LPPaMessage{
		Payload:   payload,
		Timestamp: time.Now(),
	}

	ue.lppaMessages = append(ue.lppaMessages, msg)
	if len(ue.lppaMessages) > 16 {
		ue.lppaMessages = ue.lppaMessages[len(ue.lppaMessages)-16:]
	}
}

func (ue *UeContext) GetLPPaMessages() []LPPaMessage {
	ue.lppaMu.RLock()
	defer ue.lppaMu.RUnlock()

	result := make([]LPPaMessage, len(ue.lppaMessages))
	copy(result, ue.lppaMessages)

	return result
}
