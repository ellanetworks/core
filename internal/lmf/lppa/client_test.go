// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/lppa"
)

// buildECIDResponsePDU builds an E-CIDMeasurementInitiationResponse PDU carrying
// a serving E-UTRA cell, TAC and measured results, matching what the eNB
// produces on the wire.
func buildECIDResponsePDU(t *testing.T, esmlcMeasID, enbMeasID, ta int64) []byte {
	t.Helper()

	result := &lppa.ECIDResult{
		ServingCell:        lppa.ECGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, EUTRACellID: 0x0abcde1},
		ServingCellTAC:     []byte{0x00, 0x07},
		TimingAdvanceType1: &ta,
		RSRP:               []lppa.RSRPItem{{PCI: 1, EARFCN: 100, ValueRSRP: 80}},
		RSRQ:               []lppa.RSRQItem{{PCI: 1, EARFCN: 100, ValueRSRQ: 20}},
	}

	b, err := lppa.BuildECIDMeasurementInitiationResponse(esmlcMeasID, enbMeasID, result)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementInitiationResponse: %v", err)
	}

	return b
}

func TestMatchMeasurementResponse(t *testing.T) {
	const measID = int64(7)

	base := time.Now()
	msgs := []mme.LPPaMessage{
		{Payload: buildECIDResponsePDU(t, measID, 9, 100), Timestamp: base},
	}

	t.Run("match", func(t *testing.T) {
		resp, fail := matchMeasurementResponse(msgs, measID, base.Add(-time.Second))
		if resp == nil || fail != nil {
			t.Fatalf("expected a match, got resp=%v fail=%v", resp, fail)
		}

		m := mapECIDResult(resp.Result)
		if m.TA == nil || *m.TA != 100 {
			t.Errorf("TA = %v, want 100", m.TA)
		}

		if m.RSRP == nil || *m.RSRP != -6100 {
			t.Errorf("RSRP = %v, want -6100 (dBm×100 for ValueRSRP 80)", m.RSRP)
		}

		if m.RSRQ == nil || *m.RSRQ != -1000 {
			t.Errorf("RSRQ = %v, want -1000 (dB×100 for ValueRSRQ 20)", m.RSRQ)
		}
	})

	t.Run("different measurement id falls back to newest fresh response", func(t *testing.T) {
		resp, _ := matchMeasurementResponse(msgs, 3, base.Add(-time.Second))
		if resp == nil {
			t.Fatal("expected tolerant fallback to newest fresh response, got nil")
		}
	})

	t.Run("stale message ignored", func(t *testing.T) {
		resp, fail := matchMeasurementResponse(msgs, measID, base.Add(time.Second))
		if resp != nil || fail != nil {
			t.Fatalf("expected no match for a message before notBefore, got resp=%v fail=%v", resp, fail)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if resp, fail := matchMeasurementResponse(nil, measID, base); resp != nil || fail != nil {
			t.Fatalf("expected no match on empty input, got resp=%v fail=%v", resp, fail)
		}
	})
}

func TestMatchMeasurementResponseFailure(t *testing.T) {
	const measID = int64(4)

	base := time.Now()

	failPDU, err := lppa.BuildECIDMeasurementInitiationFailure(measID, lppa.Cause{Group: lppa.CauseGroupRadioNetwork, Value: 1})
	if err != nil {
		t.Fatal(err)
	}

	msgs := []mme.LPPaMessage{{Payload: failPDU, Timestamp: base}}

	resp, fail := matchMeasurementResponse(msgs, measID, base.Add(-time.Second))
	if resp != nil || fail == nil {
		t.Fatalf("expected a failure, got resp=%v fail=%v", resp, fail)
	}

	if fail.ESMLCUEMeasurementID != measID || fail.Cause.Group != lppa.CauseGroupRadioNetwork {
		t.Fatalf("failure = %+v", fail)
	}
}

func TestEUTRAConversions(t *testing.T) {
	rsrp := []struct{ in, want int64 }{{0, -14100}, {80, -6100}, {97, -4400}}
	for _, tc := range rsrp {
		if got := valueRSRPToDBm(tc.in); int64(got) != tc.want {
			t.Errorf("valueRSRPToDBm(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}

	rsrq := []struct{ in, want int64 }{{0, -2000}, {20, -1000}, {34, -300}}
	for _, tc := range rsrq {
		if got := valueRSRQToDB(tc.in); int64(got) != tc.want {
			t.Errorf("valueRSRQToDB(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}

	// Angle of Arrival is 0.5-degree units: 180 → 90.0°.
	aoa := int64(180)
	m := mapECIDResult(&lppa.ECIDResult{AngleOfArrival: &aoa})

	if m.AoAAzimuthDegrees == nil || *m.AoAAzimuthDegrees != 90.0 {
		t.Fatalf("AoA = %v, want 90.0", m.AoAAzimuthDegrees)
	}
}
