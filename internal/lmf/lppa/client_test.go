// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"reflect"
	"testing"
	"time"

	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
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
		if len(m.Cells) != 2 {
			t.Fatalf("cells = %+v, want the measured cell and the serving cell", m.Cells)
		}

		measured, serving := m.Cells[0], m.Cells[1]

		if measured.Serving || *measured.PCI != 1 || *measured.ARFCN != 100 {
			t.Errorf("measured cell = %+v", measured)
		}

		if measured.RSRP == nil || *measured.RSRP != -61 {
			t.Errorf("RSRP = %v, want -61 dBm for ValueRSRP 80", measured.RSRP)
		}

		if measured.RSRQ == nil || *measured.RSRQ != -10 {
			t.Errorf("RSRQ = %v, want -10 dB for ValueRSRQ 20", measured.RSRQ)
		}

		if !serving.Serving || serving.ECGI == nil || serving.ECGI.EutraCellID != "0abcde1" {
			t.Errorf("serving cell = %+v", serving)
		}

		if serving.TimingAdvance == nil || *serving.TimingAdvance != 100 {
			t.Errorf("TA = %v, want 100", serving.TimingAdvance)
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

func TestMapECIDResultTagsTheServingCell(t *testing.T) {
	aoa := int64(180)
	m := mapECIDResult(&lppa.ECIDResult{
		ServingCell:    lppa.ECGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, EUTRACellID: 0x0abcde1},
		AngleOfArrival: &aoa,
		RSRP: []lppa.RSRPItem{
			{PCI: 2, EARFCN: 100, ValueRSRP: 50},
			{PCI: 1, EARFCN: 100, ECGI: &lppa.ECGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, EUTRACellID: 0x0abcde1}, ValueRSRP: 80},
		},
	})

	if len(m.Cells) != 2 {
		t.Fatalf("cells = %+v, want 2", m.Cells)
	}

	if m.Cells[0].Serving || !m.Cells[1].Serving {
		t.Fatalf("serving flags = %t, %t, want the cell naming the serving ECGI", m.Cells[0].Serving, m.Cells[1].Serving)
	}

	if az := m.Cells[1].AoAAzimuthDegrees; az == nil || *az != 90.0 {
		t.Fatalf("AoA = %v, want 90.0 for 0.5-degree unit 180", az)
	}
}

func TestMapECIDResultKeepsTheFullAccessPointPosition(t *testing.T) {
	m := mapECIDResult(&lppa.ECIDResult{APPosition: &lppa.APPosition{
		LatitudeDegrees:        48.4,
		LongitudeDegrees:       -68.6,
		DirectionOfAltitude:    1,
		Altitude:               12,
		UncertaintySemiMajor:   20,
		UncertaintySemiMinor:   10,
		OrientationOfMajorAxis: 45,
		UncertaintyAltitude:    19,
		Confidence:             68,
	}})

	want := lmfmodels.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid{
		LatitudeDegrees: 48.4, LongitudeDegrees: -68.6, AltitudeMeters: -12,
		UncertaintySemiMajor: 20, UncertaintySemiMinor: 10, OrientationMajor: 45, UncertaintyAltitude: 19, Confidence: 68,
	}.Estimate()

	if !reflect.DeepEqual(m.APPosition, want) {
		t.Errorf("APPosition = %+v, want %+v", m.APPosition, want)
	}
}
