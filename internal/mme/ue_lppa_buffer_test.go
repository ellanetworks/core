// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"testing"
)

func TestLPPaBuffer_SetThenPop(t *testing.T) {
	ue := NewUeContext()

	if got := ue.PopLPPaBuffered(); got != nil {
		t.Fatalf("expected no buffered LPPa initially, got %+v", got)
	}

	measID := int64(7)
	payload := []byte{0xde, 0xad, 0xbe, 0xef}

	ue.SetLPPaBuffered(measID, payload)

	got := ue.PopLPPaBuffered()
	if got == nil {
		t.Fatal("expected the buffered LPPa message")
	}

	if got.MeasurementID != measID {
		t.Errorf("MeasurementID = %d, want %d", got.MeasurementID, measID)
	}

	if !bytes.Equal(got.Payload, payload) {
		t.Errorf("Payload = %x, want %x", got.Payload, payload)
	}

	if got := ue.PopLPPaBuffered(); got != nil {
		t.Errorf("expected the buffer cleared by Pop, got %+v", got)
	}
}

// ClearLPPaBufferedIf discards only the measurement it names, so one request cannot drop
// another's.
func TestLPPaBuffer_ClearIfMatchesMeasurementID(t *testing.T) {
	ue := NewUeContext()

	ue.SetLPPaBuffered(7, []byte{0x01})

	ue.ClearLPPaBufferedIf(8)

	if ue.PopLPPaBuffered() == nil {
		t.Fatal("a non-matching measurement id must not clear the buffer")
	}

	ue.SetLPPaBuffered(7, []byte{0x01})

	ue.ClearLPPaBufferedIf(7)

	if got := ue.PopLPPaBuffered(); got != nil {
		t.Errorf("a matching measurement id must clear the buffer, got %+v", got)
	}
}
