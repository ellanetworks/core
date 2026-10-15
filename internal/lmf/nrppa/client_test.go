// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/nrppa"
)

// buildECIDResponsePDU builds an E-CIDMeasurementInitiationResponse PDU carrying
// a serving NR cell, TAC and a timing-advance measured result, matching what the
// gNB produces on the wire.
func buildECIDResponsePDU(t *testing.T, lmfMeasID, ranMeasID, ta int64) []byte {
	t.Helper()

	nrCell := uint64(0x123456789)

	result := &nrppa.ECIDResult{
		ServingCell: nrppa.NGRANCGI{
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
		resp, fail := matchMeasurementResponse(msgs, measID, base.Add(-time.Second))
		if resp == nil {
			t.Fatal("expected a match, got nil")
		}

		if fail != nil {
			t.Fatalf("expected no failure, got %+v", fail)
		}

		m := mapECIDResult(resp.Result)

		var serving *lmfmodels.CellMeasurement

		for i := range m.Cells {
			if m.Cells[i].Serving {
				serving = &m.Cells[i]
			}
		}

		if serving == nil || serving.TimingAdvance == nil || *serving.TimingAdvance != 100 {
			t.Errorf("expected serving cell TA=100, got %+v", serving)
		}
	})

	t.Run("different measurement id falls back to newest fresh response", func(t *testing.T) {
		// Some gNBs do not echo the LMF-assigned measurement id. Since only
		// fresh messages (>= notBefore) are considered and there is one
		// outstanding request per UE, the newest fresh response is accepted.
		resp, fail := matchMeasurementResponse(msgs, 3, base.Add(-time.Second))
		if resp == nil {
			t.Fatal("expected tolerant fallback to newest fresh response, got nil")
		}

		if fail != nil {
			t.Errorf("expected no failure, got %+v", fail)
		}

		if resp.LMFUEMeasurementID != measID {
			t.Errorf("expected fallback to response with id %d, got %d", measID, resp.LMFUEMeasurementID)
		}
	})

	t.Run("message older than notBefore", func(t *testing.T) {
		resp, fail := matchMeasurementResponse(msgs, measID, base.Add(time.Second))
		if resp != nil {
			t.Errorf("expected nil for stale message, got %+v", resp)
		}

		if fail != nil {
			t.Errorf("expected no failure, got %+v", fail)
		}
	})

	t.Run("no messages", func(t *testing.T) {
		resp, fail := matchMeasurementResponse(nil, measID, base)
		if resp != nil {
			t.Errorf("expected nil for empty message set, got %+v", resp)
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

		resp, fail := matchMeasurementResponse(failMsgs, measID, base.Add(-time.Second))
		if resp != nil {
			t.Errorf("expected nil measurements for failure, got %+v", resp)
		}

		if fail == nil {
			t.Fatal("expected a failure, got nil")
		}

		if fail.LMFUEMeasurementID != measID {
			t.Errorf("expected LMFUEMeasurementID=%d, got %d", measID, fail.LMFUEMeasurementID)
		}
	})

	t.Run("failure with different measurement id still reported", func(t *testing.T) {
		// 9 is a valid UE-Measurement-ID (root range 1..15) other than measID.
		failPDU, err := nrppa.BuildECIDMeasurementInitiationFailure(9, nrppa.Cause{Group: nrppa.CauseGroupRadioNetwork, Value: 0})
		if err != nil {
			t.Fatalf("BuildECIDMeasurementInitiationFailure: %v", err)
		}

		failMsgs := []amf.NRPPaMessage{
			{Payload: failPDU, Timestamp: base},
		}

		resp, fail := matchMeasurementResponse(failMsgs, measID, base.Add(-time.Second))
		if resp != nil {
			t.Errorf("expected nil response, got %+v", resp)
		}

		if fail == nil {
			t.Fatal("expected tolerant fallback to newest fresh failure, got nil")
		}
	})
}

func TestMapECIDResultKeepsCellsAndBeams(t *testing.T) {
	plmn := []byte{0x00, 0xf1, 0x10}
	servingID := uint64(0x000000fc1)
	cell := int64(101)
	quality := int64(66)

	m := mapECIDResult(&nrppa.ECIDResult{
		ServingCell: nrppa.NGRANCGI{PLMNIdentity: plmn, NRCellIdentity: &servingID},
		SSRSRP: []nrppa.SSRSRPItem{
			{NRPCI: 180, NRARFCN: 662592, CGI: &nrppa.CGINR{PLMNIdentity: plmn, NRCellIdentity: servingID}, Value: &cell, PerSSB: []nrppa.SSBResultItem{{SSBIndex: 3, Value: 90}}},
			{NRPCI: 181, NRARFCN: 662592, Value: &cell},
		},
		SSRSRQ: []nrppa.SSRSRQItem{
			{NRPCI: 180, NRARFCN: 662592, Value: &quality, PerSSB: []nrppa.SSBResultItem{{SSBIndex: 3, Value: 60}}},
		},
	})

	if len(m.Cells) != 2 {
		t.Fatalf("cells = %+v, want 2", m.Cells)
	}

	serving := m.Cells[0]
	if !serving.Serving || serving.Source != lmfmodels.MeasurementSourceNetwork || serving.RAT != lmfmodels.RATNR {
		t.Fatalf("serving cell = %+v", serving)
	}

	if *serving.SSRSRP != -56 || *serving.SSRSRQ != -10.5 {
		t.Errorf("SS-RSRP/RSRQ = %v/%v, want -56/-10.5", *serving.SSRSRP, *serving.SSRSRQ)
	}

	if len(serving.SSBBeams) != 1 || serving.SSBBeams[0].Index != 3 || *serving.SSBBeams[0].RSRP != -67 || *serving.SSBBeams[0].RSRQ != -13.5 {
		t.Errorf("SSB beams = %+v", serving.SSBBeams)
	}

	if m.Cells[1].Serving || *m.Cells[1].PCI != 181 {
		t.Errorf("neighbour cell = %+v", m.Cells[1])
	}
}

func TestEncodeMeasurementQuantitiesCoversEveryNRValue(t *testing.T) {
	for _, q := range lmfmodels.NRMeasurementQuantities {
		if _, err := encodeMeasurementQuantities([]lmfmodels.MeasurementQuantity{q}); err != nil {
			t.Errorf("%s: %v", q, err)
		}
	}
}

func TestRequestCarriesTheRequestedQuantities(t *testing.T) {
	for _, tc := range []struct {
		name       string
		quantities []lmfmodels.MeasurementQuantity
		want       []nrppa.MeasurementQuantityValue
	}{
		{
			name:       "default",
			quantities: nil,
			want: []nrppa.MeasurementQuantityValue{
				nrppa.MeasRSRP, nrppa.MeasRSRQ, nrppa.MeasSSRSRP, nrppa.MeasSSRSRQ, nrppa.MeasCSIRSRP,
				nrppa.MeasCSIRSRQ, nrppa.MeasAngleOfArrivalNR, nrppa.MeasTimingAdvanceNR, nrppa.MeasUERxTxTimeDiff,
			},
		},
		{
			name: "explicit",
			quantities: []lmfmodels.MeasurementQuantity{
				lmfmodels.MeasurementQuantitySSRSRP,
				lmfmodels.MeasurementQuantityTimingAdvanceNR,
			},
			want: []nrppa.MeasurementQuantityValue{nrppa.MeasSSRSRP, nrppa.MeasTimingAdvanceNR},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values, err := encodeMeasurementQuantities(tc.quantities)
			if err != nil {
				t.Fatalf("encodeMeasurementQuantities: %v", err)
			}

			pdu, err := nrppa.BuildECIDMeasurementInitiationRequest(1, values)
			if err != nil {
				t.Fatalf("BuildECIDMeasurementInitiationRequest: %v", err)
			}

			parsed, err := nrppa.ParsePDU(pdu)
			if err != nil {
				t.Fatalf("ParsePDU: %v", err)
			}

			if !slices.Equal(parsed.Request.MeasurementQuantities, tc.want) {
				t.Errorf("quantities = %v, want %v", parsed.Request.MeasurementQuantities, tc.want)
			}
		})
	}
}
