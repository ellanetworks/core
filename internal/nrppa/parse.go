// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"fmt"

	"github.com/ellanetworks/core/internal/nrppa/nrppatype"
	"github.com/free5gc/aper"
)

// ParsePDU decodes an NRPPa-PDU and returns a discriminated, caller-facing
// view of the three E-CID Measurement Initiation messages. Unknown procedures
// and message types yield Kind == KindUnknown without error.
func ParsePDU(b []byte) (*ParsedPDU, error) {
	pdu, err := Decoder(b)
	if err != nil {
		return nil, fmt.Errorf("decode NRPPa-PDU: %w", err)
	}

	out := &ParsedPDU{Kind: KindUnknown}

	switch pdu.Present {
	case nrppatype.NRPPaPDUPresentInitiatingMessage:
		im := pdu.InitiatingMessage
		if im == nil || im.ProcedureCode.Value != nrppatype.ProcedureCodeECIDMeasurementInitiation {
			return out, nil
		}

		req := im.Value.ECIDMeasurementInitiationRequest
		if req == nil {
			return out, nil
		}

		out.Kind = KindECIDMeasurementInitiationRequest
		out.Request = parseRequest(req)

	case nrppatype.NRPPaPDUPresentSuccessfulOutcome:
		so := pdu.SuccessfulOutcome
		if so == nil || so.ProcedureCode.Value != nrppatype.ProcedureCodeECIDMeasurementInitiation {
			return out, nil
		}

		resp := so.Value.ECIDMeasurementInitiationResponse
		if resp == nil {
			return out, nil
		}

		out.Kind = KindECIDMeasurementInitiationResponse
		out.Response = parseResponse(resp)

	case nrppatype.NRPPaPDUPresentUnsuccessfulOutcome:
		uo := pdu.UnsuccessfulOutcome
		if uo == nil || uo.ProcedureCode.Value != nrppatype.ProcedureCodeECIDMeasurementInitiation {
			return out, nil
		}

		fail := uo.Value.ECIDMeasurementInitiationFailure
		if fail == nil {
			return out, nil
		}

		out.Kind = KindECIDMeasurementInitiationFailure
		out.Failure = parseFailure(fail)
	}

	return out, nil
}

// ParseECIDMeasurementInitiationRequest decodes a PDU and returns the request,
// or an error if the PDU is not an E-CIDMeasurementInitiationRequest.
func ParseECIDMeasurementInitiationRequest(b []byte) (*ECIDRequest, error) {
	parsed, err := ParsePDU(b)
	if err != nil {
		return nil, err
	}

	if parsed.Kind != KindECIDMeasurementInitiationRequest || parsed.Request == nil {
		return nil, fmt.Errorf("PDU is not an E-CIDMeasurementInitiationRequest (kind=%d)", parsed.Kind)
	}

	return parsed.Request, nil
}

func parseRequest(req *nrppatype.ECIDMeasurementInitiationRequest) *ECIDRequest {
	out := &ECIDRequest{}

	for i := range req.ProtocolIEs.List {
		ie := &req.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case nrppatype.ProtocolIEIDLMFUEMeasurementID:
			if v := ie.Value.LMFUEMeasurementID; v != nil {
				out.LMFUEMeasurementID = v.Value
			}
		case nrppatype.ProtocolIEIDReportCharacteristics:
			if v := ie.Value.ReportCharacteristics; v != nil {
				out.ReportCharacteristics = int(v.Value)
			}
		case nrppatype.ProtocolIEIDMeasurementQuantities:
			if v := ie.Value.MeasurementQuantities; v != nil {
				for j := range v.List {
					item := v.List[j].Value.MeasurementQuantitiesItem
					if item != nil {
						out.MeasurementQuantities = append(out.MeasurementQuantities,
							MeasurementQuantityValue(item.MeasurementQuantitiesValue.Value))
					}
				}
			}
		}
	}

	return out
}

func parseResponse(resp *nrppatype.ECIDMeasurementInitiationResponse) *ECIDResponse {
	out := &ECIDResponse{}

	for i := range resp.ProtocolIEs.List {
		ie := &resp.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case nrppatype.ProtocolIEIDLMFUEMeasurementID:
			if v := ie.Value.LMFUEMeasurementID; v != nil {
				out.LMFUEMeasurementID = v.Value
			}
		case nrppatype.ProtocolIEIDRANUEMeasurementID:
			if v := ie.Value.RANUEMeasurementID; v != nil {
				out.RANUEMeasurementID = v.Value
			}
		case nrppatype.ProtocolIEIDECIDMeasurementResult:
			if v := ie.Value.ECIDMeasurementResult; v != nil {
				out.Result = parseECIDMeasurementResult(v)
			}
		case nrppatype.ProtocolIEIDCellPortionID:
			if v := ie.Value.CellPortionID; v != nil {
				cp := v.Value
				out.CellPortionID = &cp
			}
		}
	}

	return out
}

func parseFailure(fail *nrppatype.ECIDMeasurementInitiationFailure) *ECIDFailure {
	out := &ECIDFailure{}

	for i := range fail.ProtocolIEs.List {
		ie := &fail.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case nrppatype.ProtocolIEIDLMFUEMeasurementID:
			if v := ie.Value.LMFUEMeasurementID; v != nil {
				out.LMFUEMeasurementID = v.Value
			}
		case nrppatype.ProtocolIEIDCause:
			if v := ie.Value.Cause; v != nil {
				out.Cause = parseCause(v)
			}
		}
	}

	return out
}

func parseCause(c *nrppatype.Cause) Cause {
	switch c.Present {
	case nrppatype.CausePresentRadioNetwork:
		if c.RadioNetwork != nil {
			return Cause{Group: CauseGroupRadioNetwork, Value: int64(c.RadioNetwork.Value)}
		}
	case nrppatype.CausePresentProtocol:
		if c.Protocol != nil {
			return Cause{Group: CauseGroupProtocol, Value: int64(c.Protocol.Value)}
		}
	case nrppatype.CausePresentMisc:
		if c.Misc != nil {
			return Cause{Group: CauseGroupMisc, Value: int64(c.Misc.Value)}
		}
	case nrppatype.CausePresentChoiceExtension:
		return Cause{Group: CauseGroupChoiceExtension}
	}

	return Cause{Group: CauseGroupRadioNetwork, Value: int64(nrppatype.CauseRadioNetworkPresentUnspecified)}
}

func parseECIDMeasurementResult(mr *nrppatype.ECIDMeasurementResult) *ECIDResult {
	out := &ECIDResult{
		ServingCellTAC: []byte(mr.ServingCellTAC.Value),
	}

	out.ServingCell.PLMNIdentity = []byte(mr.ServingCellID.PLMNIdentity.Value)

	switch mr.ServingCellID.NGRANCell.Present {
	case nrppatype.NGRANCellPresentNRCellID:
		if v := mr.ServingCellID.NGRANCell.NRCellID; v != nil {
			id := bitStringToUint64(v.Value)
			out.ServingCell.NRCellIdentity = &id
		}
	case nrppatype.NGRANCellPresentEUTRACellID:
		if v := mr.ServingCellID.NGRANCell.EUTRACellID; v != nil {
			id := bitStringToUint64(v.Value)
			out.ServingCell.EUTRACellID = &id
		}
	}

	if mr.NGRANAccessPointPosition != nil {
		out.APPosition = parseAccessPointPosition(mr.NGRANAccessPointPosition)
	}

	if mr.MeasuredResults != nil {
		for i := range mr.MeasuredResults.List {
			v := &mr.MeasuredResults.List[i]

			switch v.Present {
			case nrppatype.MeasuredResultsValuePresentValueTimingAdvanceType1EUTRA:
				if v.ValueTimingAdvanceType1EUTRA != nil {
					ta := *v.ValueTimingAdvanceType1EUTRA
					out.TimingAdvanceType1 = &ta
				}
			case nrppatype.MeasuredResultsValuePresentValueTimingAdvanceType2EUTRA:
				if v.ValueTimingAdvanceType2EUTRA != nil {
					ta := *v.ValueTimingAdvanceType2EUTRA
					out.TimingAdvanceType2 = &ta
				}
			}
		}
	}

	return out
}

func parseAccessPointPosition(p *nrppatype.NGRANAccessPointPosition) *APPosition {
	out := &APPosition{
		LatitudeSign:           int(p.LatitudeSign),
		Latitude:               p.Latitude,
		Longitude:              p.Longitude,
		DirectionOfAltitude:    int(p.DirectionOfAltitude),
		Altitude:               p.Altitude,
		UncertaintySemiMajor:   p.UncertaintySemiMajor,
		UncertaintySemiMinor:   p.UncertaintySemiMinor,
		OrientationOfMajorAxis: p.OrientationOfMajorAxis,
		UncertaintyAltitude:    p.UncertaintyAltitude,
		Confidence:             p.Confidence,
	}

	// TS 23.032 ellipsoid point with uncertainty ellipse:
	//   latitude  X = N * 90 / 2^23  (sign: 0 = north, 1 = south)
	//   longitude X = N * 360 / 2^24 (N is a signed 2's-complement value)
	lat := float64(p.Latitude) * 90.0 / 8388608.0 // 2^23
	if p.LatitudeSign == nrppatype.NGRANAccessPointPositionLatitudeSignSouth {
		lat = -lat
	}

	out.LatitudeDegrees = lat
	out.LongitudeDegrees = float64(p.Longitude) * 360.0 / 16777216.0 // 2^24

	return out
}

// bitStringToUint64 reads the (top-aligned) effective bits of a BitString into
// a right-aligned uint64.
func bitStringToUint64(bs aper.BitString) uint64 {
	var v uint64

	for _, b := range bs.Bytes {
		v = (v << 8) | uint64(b)
	}

	// The value is stored left-aligned within the byte buffer; shift the unused
	// low bits back out.
	totalBits := uint64(len(bs.Bytes) * 8)
	if totalBits > bs.BitLength {
		v >>= totalBits - bs.BitLength
	}

	return v
}
