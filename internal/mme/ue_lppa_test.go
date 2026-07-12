// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"testing"

	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
)

func TestLPPaMessageRingBuffer(t *testing.T) {
	ue := &UeContext{}

	if got := ue.GetLPPaMessages(); len(got) != 0 {
		t.Fatalf("empty buffer returned %d messages", len(got))
	}

	for i := 0; i < 20; i++ {
		ue.SetLPPaMessage([]byte{byte(i)})
	}

	msgs := ue.GetLPPaMessages()
	if len(msgs) != 16 {
		t.Fatalf("buffer kept %d messages, want 16", len(msgs))
	}

	// The oldest four (0..3) are evicted; the window is 4..19.
	if msgs[0].Payload[0] != 4 || msgs[15].Payload[0] != 19 {
		t.Fatalf("window = [%d..%d], want [4..19]", msgs[0].Payload[0], msgs[15].Payload[0])
	}
}

func TestSetLPPaMessageCopiesInput(t *testing.T) {
	ue := &UeContext{}

	data := []byte{1, 2, 3}
	ue.SetLPPaMessage(data)
	data[0] = 9

	if got := ue.GetLPPaMessages(); !bytes.Equal(got[0].Payload, []byte{1, 2, 3}) {
		t.Fatalf("stored payload mutated with caller's slice: %v", got[0].Payload)
	}
}

func TestRadioMeasurementsCopy(t *testing.T) {
	ue := &UeContext{}

	if ue.GetRadioMeasurements() != nil {
		t.Fatal("expected nil measurements initially")
	}

	ue.SetRadioMeasurements(nil) // no-op

	if ue.GetRadioMeasurements() != nil {
		t.Fatal("SetRadioMeasurements(nil) should be a no-op")
	}

	rsrp := int32(-8000)
	ue.SetRadioMeasurements(&lmfmodels.RadioMeasurements{RSRP: &rsrp})

	got := ue.GetRadioMeasurements()
	if got == nil || got.RSRP == nil || *got.RSRP != -8000 {
		t.Fatalf("measurements = %+v", got)
	}

	// Each Get returns a distinct struct so a caller cannot mutate the stored one.
	if other := ue.GetRadioMeasurements(); other == got {
		t.Fatal("GetRadioMeasurements returned the same struct pointer")
	}
}
