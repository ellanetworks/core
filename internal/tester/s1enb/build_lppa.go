// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/lppa"
	"github.com/ellanetworks/core/s1ap"
)

// Sample E-CID measurements the eNB reports, anchored near 45.0°N, ~21.5°E to
// match the tester's mock coordinates.
const (
	sampleLPPaTimingAdvance  int64 = 100 // valueTimingAdvanceType1 (0..7690)
	sampleLPPaAngleOfArrival int64 = 180 // 0.5° units → 90.0°
	sampleLPPaValueRSRP      int64 = 80  // 0..97 (TS 36.133 §9.1.4)
	sampleLPPaValueRSRQ      int64 = 20  // 0..34 (TS 36.133 §9.1.7)
)

func (e *ENB) sampleECIDResult() *lppa.ECIDResult {
	cgi := e.eutranCGI()
	ta := sampleLPPaTimingAdvance
	aoa := sampleLPPaAngleOfArrival

	tac := make([]byte, 2)
	binary.BigEndian.PutUint16(tac, e.tac)

	return &lppa.ECIDResult{
		ServingCell:    lppa.ECGI{PLMNIdentity: cgi.PLMNIdentity[:], EUTRACellID: uint64(cgi.CellID)},
		ServingCellTAC: tac,
		APPosition: &lppa.APPosition{
			LatitudeSign:           0,       // north
			Latitude:               4194304, // 2^22 → 45.0° via N·90/2^23
			Longitude:              1000000, // ~21.45° via N·360/2^24
			DirectionOfAltitude:    0,       // height
			Altitude:               100,
			UncertaintySemiMajor:   5,
			UncertaintySemiMinor:   5,
			OrientationOfMajorAxis: 0,
			UncertaintyAltitude:    3,
			Confidence:             67,
		},
		TimingAdvanceType1: &ta,
		AngleOfArrival:     &aoa,
		RSRP:               []lppa.RSRPItem{{PCI: 1, EARFCN: 100, ValueRSRP: sampleLPPaValueRSRP}},
		RSRQ:               []lppa.RSRQItem{{PCI: 1, EARFCN: 100, ValueRSRQ: sampleLPPaValueRSRQ}},
	}
}

// BuildUplinkLPPaECIDResponse wraps an LPPa E-CIDMeasurementInitiationResponse in
// an S1AP Uplink UE-Associated LPPa Transport, echoing the request's IDs.
func (e *ENB) BuildUplinkLPPaECIDResponse(mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, routingID s1ap.RoutingID, esmlcMeasID int64) ([]byte, error) {
	pdu, err := lppa.BuildECIDMeasurementInitiationResponse(esmlcMeasID, 1, e.sampleECIDResult())
	if err != nil {
		return nil, fmt.Errorf("build LPPa E-CID response: %w", err)
	}

	return (&s1ap.UplinkUEAssociatedLPPaTransport{
		MMEUES1APID: mmeUEID,
		ENBUES1APID: enbUEID,
		RoutingID:   routingID,
		LPPaPDU:     pdu,
	}).Marshal()
}
