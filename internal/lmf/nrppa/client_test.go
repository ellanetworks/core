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
		m, fail := matchMeasurementResponse(msgs, measID, base.Add(-time.Second))
		if m == nil {
			t.Fatal("expected a match, got nil")
		}

		if fail != nil {
			t.Fatalf("expected no failure, got %+v", fail)
		}

		if m.TA == nil || *m.TA != 100 {
			t.Errorf("expected TA=100, got %v", m.TA)
		}
	})

	t.Run("wrong measurement id", func(t *testing.T) {
		m, fail := matchMeasurementResponse(msgs, 3, base.Add(-time.Second))
		if m != nil {
			t.Errorf("expected nil for non-matching measurement ID, got %+v", m)
		}

		if fail != nil {
			t.Errorf("expected no failure, got %+v", fail)
		}
	})

	t.Run("message older than notBefore", func(t *testing.T) {
		m, fail := matchMeasurementResponse(msgs, measID, base.Add(time.Second))
		if m != nil {
			t.Errorf("expected nil for stale message, got %+v", m)
		}

		if fail != nil {
			t.Errorf("expected no failure, got %+v", fail)
		}
	})

	t.Run("no messages", func(t *testing.T) {
		m, fail := matchMeasurementResponse(nil, measID, base)
		if m != nil {
			t.Errorf("expected nil for empty message set, got %+v", m)
		}

		if fail != nil {
			t.Errorf("expected no failure, got %+v", fail)
		}
	})

	t.Run("failure response", func(t *testing.T) {
		failPDU, err := nrppa.BuildECIDMeasurementInitiationFailure(measID, nrppa.Cause{Group: nrppa.CauseGroupRadioNetwork, Value: 0})
		if err != nil {
			t.Fatalf("BuildECIDMeasurementInitiationFailure: %v", err)
		}

		failMsgs := []amf.NRPPaMessage{
			{Payload: failPDU, Timestamp: base},
		}

		m, fail := matchMeasurementResponse(failMsgs, measID, base.Add(-time.Second))
		if m != nil {
			t.Errorf("expected nil measurements for failure, got %+v", m)
		}

		if fail == nil {
			t.Fatal("expected a failure, got nil")
		}

		if fail.LMFUEMeasurementID != measID {
			t.Errorf("expected LMFUEMeasurementID=%d, got %d", measID, fail.LMFUEMeasurementID)
		}
	})

	t.Run("failure with wrong measurement id", func(t *testing.T) {
		failPDU, err := nrppa.BuildECIDMeasurementInitiationFailure(99, nrppa.Cause{Group: nrppa.CauseGroupRadioNetwork, Value: 0})
		if err != nil {
			t.Fatalf("BuildECIDMeasurementInitiationFailure: %v", err)
		}

		failMsgs := []amf.NRPPaMessage{
			{Payload: failPDU, Timestamp: base},
		}

		m, fail := matchMeasurementResponse(failMsgs, measID, base.Add(-time.Second))
		if m != nil {
			t.Errorf("expected nil for non-matching measurement ID, got %+v", m)
		}

		if fail != nil {
			t.Errorf("expected no failure for non-matching ID, got %+v", fail)
		}
	})
}

// TestNRRSRPRSRQConversions locks in the NR (not E-UTRA) SS-/CSI- RSRP/RSRQ
// report-value → dBm/dB mappings from TS 38.133. The RSRP=101 / RSRQ=66 cases
// correspond to the real gNB capture used to validate E-CID decoding.
func TestNRRSRPRSRQConversions(t *testing.T) {
	t.Run("SS-RSRP", func(t *testing.T) {
		cases := map[int64]int32{
			0:   -15600, // < -156 dBm
			1:   -15600, // -156 dBm
			101: -5600,  // -56 dBm (captured value)
			127: -3000,  // -30 dBm
		}
		for v, want := range cases {
			if got := ssrsrpToDBm(v); got != want {
				t.Errorf("ssrsrpToDBm(%d) = %d, want %d", v, got, want)
			}
		}
	})

	t.Run("SS-RSRQ", func(t *testing.T) {
		cases := map[int64]int32{
			0:   -4300, // < -43 dB
			1:   -4300, // -43 dB
			66:  -1050, // -10.5 dB (captured value)
			127: 2000,  // +20 dB
		}
		for v, want := range cases {
			if got := ssrsrqToDB(v); got != want {
				t.Errorf("ssrsrqToDB(%d) = %d, want %d", v, got, want)
			}
		}
	})

	t.Run("CSI shares SS mapping", func(t *testing.T) {
		if got, want := csirsrpToDBm(101), ssrsrpToDBm(101); got != want {
			t.Errorf("csirsrpToDBm(101) = %d, want %d", got, want)
		}

		if got, want := csirsrqToDB(66), ssrsrqToDB(66); got != want {
			t.Errorf("csirsrqToDB(66) = %d, want %d", got, want)
		}
	})
}
