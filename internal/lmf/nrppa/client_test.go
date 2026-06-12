// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/nrppa"
)

// buildECIDResponsePDU builds an E-CIDMeasurementInitiationResponse PDU carrying
// a serving NR cell, TAC and a timing-advance measured result, matching what the
// gNB produces on the wire.
func buildECIDResponsePDU(t *testing.T, lmfMeasID, ranMeasID, ta int64) []byte {
	t.Helper()

	nrCell := uint64(0x123456789)

	result := &nrppa.ECIDResult{
		ServingCell: nrppa.ServingCell{
			PLMNIdentity:   []byte{0x00, 0xf1, 0x10},
			NRCellIdentity: &nrCell,
		},
		ServingCellTAC:     []byte{0x00, 0x00, 0x01},
		TimingAdvanceType1: &ta,
	}

	b, err := nrppa.BuildECIDMeasurementInitiationResponse(lmfMeasID, ranMeasID, result)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementInitiationResponse: %v", err)
	}

	return b
}

func TestMatchMeasurementResponse(t *testing.T) {
	const measID = int64(7)

	base := time.Now()
	msgs := []amf.NRPPaMessage{
		{Payload: buildECIDResponsePDU(t, measID, 9, 100), Timestamp: base},
	}

	t.Run("match", func(t *testing.T) {
		m := matchMeasurementResponse(msgs, measID, base.Add(-time.Second))
		if m == nil {
			t.Fatal("expected a match, got nil")
		}

		if m.TA == nil || *m.TA != 100 {
			t.Errorf("expected TA=100, got %v", m.TA)
		}
	})

	t.Run("wrong measurement id", func(t *testing.T) {
		if m := matchMeasurementResponse(msgs, 3, base.Add(-time.Second)); m != nil {
			t.Errorf("expected nil for non-matching measurement ID, got %+v", m)
		}
	})

	t.Run("message older than notBefore", func(t *testing.T) {
		if m := matchMeasurementResponse(msgs, measID, base.Add(time.Second)); m != nil {
			t.Errorf("expected nil for stale message, got %+v", m)
		}
	})

	t.Run("no messages", func(t *testing.T) {
		if m := matchMeasurementResponse(nil, measID, base); m != nil {
			t.Errorf("expected nil for empty message set, got %+v", m)
		}
	})
}
