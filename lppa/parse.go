// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

const maxProtocolExtensions = 65535

// ParsePDU decodes an LPPa-PDU and dispatches on its procedure code, returning
// the E-CID message it carries. Unrecognised procedures yield KindUnknown with
// no error so a caller can ignore them.
func ParsePDU(b []byte) (*ParsedPDU, error) {
	msg, err := unmarshalPDU(b)
	if err != nil {
		return nil, err
	}

	switch msg.choiceIndex {
	case pduInitiatingMessage:
		switch msg.procedureCode {
		case ProcECIDMeasurementInitiation:
			req, err := parseRequest(msg.value)
			if err != nil {
				return nil, err
			}

			return &ParsedPDU{Kind: KindECIDMeasurementInitiationRequest, Request: req}, nil
		case ProcECIDMeasurementTermination:
			term, err := parseTermination(msg.value)
			if err != nil {
				return nil, err
			}

			return &ParsedPDU{Kind: KindECIDMeasurementTerminationCommand, Termination: term}, nil
		case ProcECIDMeasurementFailureIndication:
			fi, err := parseFailureIndication(msg.value)
			if err != nil {
				return nil, err
			}

			return &ParsedPDU{Kind: KindECIDMeasurementFailureIndication, FailureIndication: fi}, nil
		}
	case pduSuccessfulOutcome:
		if msg.procedureCode == ProcECIDMeasurementInitiation {
			resp, err := parseResponse(msg.value)
			if err != nil {
				return nil, err
			}

			return &ParsedPDU{Kind: KindECIDMeasurementInitiationResponse, Response: resp}, nil
		}
	case pduUnsuccessfulOutcome:
		if msg.procedureCode == ProcECIDMeasurementInitiation {
			fail, err := parseFailure(msg.value)
			if err != nil {
				return nil, err
			}

			return &ParsedPDU{Kind: KindECIDMeasurementInitiationFailure, Failure: fail}, nil
		}
	}

	return &ParsedPDU{Kind: KindUnknown}, nil
}

// decodeMessageIEs reads an E-CID message SEQUENCE preamble and its
// ProtocolIE-Container, returning the fields for id dispatch.
func decodeMessageIEs(value []byte) ([]rawIE, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("lppa: message preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	return fields, nil
}

func parseRequest(value []byte) (*ECIDRequest, error) {
	fields, err := decodeMessageIEs(value)
	if err != nil {
		return nil, err
	}

	req := &ECIDRequest{}

	var seenID, seenReport, seenQuantities bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idESMLCUEMeasurementID:
			req.ESMLCUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenID = true
		case idReportCharacteristics:
			var idx int

			idx, _, err = sub.ReadEnum(reportCharacteristicsRootCount, true)
			req.ReportCharacteristics = idx
			seenReport = true
		case idMeasurementQuantities:
			req.MeasurementQuantities, err = decodeMeasurementQuantities(sub)
			seenQuantities = true
		}

		if err != nil {
			return nil, fmt.Errorf("lppa: request IE %d: %w", f.id, err)
		}
	}

	if !seenID || !seenReport || !seenQuantities {
		return nil, fmt.Errorf("lppa: E-CID request missing mandatory IE")
	}

	return req, nil
}

func decodeMeasurementQuantities(r *aper.Reader) ([]MeasurementQuantityValue, error) {
	n, err := r.ReadConstrainedLength(1, maxNoMeas)
	if err != nil {
		return nil, err
	}

	out := make([]MeasurementQuantityValue, 0, n)

	for i := 0; i < n; i++ {
		if _, err := r.ReadConstrainedInt(0, maxProtocolIEs); err != nil {
			return nil, err
		}

		if _, _, err := r.ReadEnum(criticalityRootCount, false); err != nil {
			return nil, err
		}

		item, err := r.ReadOpenType()
		if err != nil {
			return nil, err
		}

		ir := aper.NewReader(item)

		extPresent, opt, err := ir.ReadSequencePreamble(true, 1)
		if err != nil {
			return nil, err
		}

		idx, _, err := ir.ReadEnum(measurementQuantityRootCount, true)
		if err != nil {
			return nil, err
		}

		if err := skipSequenceTail(ir, opt[0], extPresent); err != nil {
			return nil, err
		}

		out = append(out, MeasurementQuantityValue(idx))
	}

	return out, nil
}

func parseResponse(value []byte) (*ECIDResponse, error) {
	fields, err := decodeMessageIEs(value)
	if err != nil {
		return nil, err
	}

	resp := &ECIDResponse{}

	var seenESMLC, seenENB bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idESMLCUEMeasurementID:
			resp.ESMLCUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenESMLC = true
		case idENBUEMeasurementID:
			resp.ENBUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenENB = true
		case idECIDMeasurementResult:
			resp.Result, err = decodeMeasurementResult(sub)
		case idCellPortionID:
			var v int64

			v, err = readExtConstrainedInt(sub, 0, 255)
			resp.CellPortionID = &v
		}

		if err != nil {
			return nil, fmt.Errorf("lppa: response IE %d: %w", f.id, err)
		}
	}

	if !seenESMLC || !seenENB {
		return nil, fmt.Errorf("lppa: E-CID response missing mandatory IE")
	}

	return resp, nil
}

func decodeMeasurementResult(r *aper.Reader) (*ECIDResult, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 2)
	if err != nil {
		return nil, err
	}

	res := &ECIDResult{}

	res.ServingCell, err = decodeECGI(r)
	if err != nil {
		return nil, err
	}

	res.ServingCellTAC, err = r.ReadOctetString(2, 2, false)
	if err != nil {
		return nil, err
	}

	if opt[0] {
		res.APPosition, err = decodeAPPosition(r)
		if err != nil {
			return nil, err
		}
	}

	if opt[1] {
		if err := decodeMeasuredResults(r, res); err != nil {
			return nil, err
		}
	}

	if err := skipSequenceTail(r, false, extPresent); err != nil {
		return nil, err
	}

	return res, nil
}

func decodeECGI(r *aper.Reader) (ECGI, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ECGI{}, err
	}

	plmn, err := r.ReadOctetString(3, 3, false)
	if err != nil {
		return ECGI{}, err
	}

	bits, _, err := r.ReadBitString(28, 28, false)
	if err != nil {
		return ECGI{}, err
	}

	if err := skipSequenceTail(r, opt[0], extPresent); err != nil {
		return ECGI{}, err
	}

	return ECGI{PLMNIdentity: plmn, EUTRACellID: bitsToUint(bits, 28)}, nil
}

func decodeAPPosition(r *aper.Reader) (*APPosition, error) {
	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, err
	}

	p := &APPosition{}

	latSign, _, err := r.ReadEnum(2, false)
	if err != nil {
		return nil, err
	}

	p.LatitudeSign = latSign

	if p.Latitude, err = r.ReadConstrainedInt(0, 8388607); err != nil {
		return nil, err
	}

	if p.Longitude, err = r.ReadConstrainedInt(-8388608, 8388607); err != nil {
		return nil, err
	}

	dirAlt, _, err := r.ReadEnum(2, false)
	if err != nil {
		return nil, err
	}

	p.DirectionOfAltitude = dirAlt

	for _, dst := range []struct {
		p      *int64
		lb, ub int64
	}{
		{&p.Altitude, 0, 32767},
		{&p.UncertaintySemiMajor, 0, 127},
		{&p.UncertaintySemiMinor, 0, 127},
		{&p.OrientationOfMajorAxis, 0, 179},
		{&p.UncertaintyAltitude, 0, 127},
		{&p.Confidence, 0, 100},
	} {
		if *dst.p, err = r.ReadConstrainedInt(dst.lb, dst.ub); err != nil {
			return nil, err
		}
	}

	if err := skipSequenceTail(r, false, extPresent); err != nil {
		return nil, err
	}

	p.LatitudeDegrees, p.LongitudeDegrees = apToDegrees(p)

	return p, nil
}

// apToDegrees converts the TS 23.032 encoded latitude/longitude to WGS-84
// decimal degrees: N = round(abs(x) * 2^23 / 90) for latitude,
// N = round(x * 2^24 / 360) for longitude.
func apToDegrees(p *APPosition) (lat, lon float64) {
	lat = float64(p.Latitude) * 90.0 / 8388608.0
	if p.LatitudeSign == 1 {
		lat = -lat
	}

	lon = float64(p.Longitude) * 360.0 / 16777216.0

	return lat, lon
}

func decodeMeasuredResults(r *aper.Reader, res *ECIDResult) error {
	n, err := r.ReadConstrainedLength(1, maxNoMeas)
	if err != nil {
		return err
	}

	for i := 0; i < n; i++ {
		idx, isExt, err := r.ReadChoiceIndex(measuredResultsRootCount, true)
		if err != nil {
			return err
		}

		if isExt {
			if _, err := r.ReadOpenType(); err != nil {
				return err
			}

			continue
		}

		switch idx {
		case 0:
			v, err := r.ReadConstrainedInt(0, 719)
			if err != nil {
				return err
			}

			res.AngleOfArrival = &v
		case 1:
			v, err := r.ReadConstrainedInt(0, 7690)
			if err != nil {
				return err
			}

			res.TimingAdvanceType1 = &v
		case 2:
			v, err := r.ReadConstrainedInt(0, 7690)
			if err != nil {
				return err
			}

			res.TimingAdvanceType2 = &v
		case 3:
			res.RSRP, err = decodeResultRSRP(r)
			if err != nil {
				return err
			}
		case 4:
			res.RSRQ, err = decodeResultRSRQ(r)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func decodeResultRSRP(r *aper.Reader) ([]RSRPItem, error) {
	n, err := r.ReadConstrainedLength(1, maxCellReport)
	if err != nil {
		return nil, err
	}

	out := make([]RSRPItem, 0, n)

	for i := 0; i < n; i++ {
		extPresent, opt, err := r.ReadSequencePreamble(true, 2)
		if err != nil {
			return nil, err
		}

		var it RSRPItem

		if it.PCI, err = readExtConstrainedInt(r, 0, 503); err != nil {
			return nil, err
		}

		if it.EARFCN, err = readExtConstrainedInt(r, 0, 65535); err != nil {
			return nil, err
		}

		if opt[0] {
			ecgi, err := decodeECGI(r)
			if err != nil {
				return nil, err
			}

			it.ECGI = &ecgi
		}

		if it.ValueRSRP, err = readExtConstrainedInt(r, 0, 97); err != nil {
			return nil, err
		}

		if err := skipSequenceTail(r, opt[1], extPresent); err != nil {
			return nil, err
		}

		out = append(out, it)
	}

	return out, nil
}

func decodeResultRSRQ(r *aper.Reader) ([]RSRQItem, error) {
	n, err := r.ReadConstrainedLength(1, maxCellReport)
	if err != nil {
		return nil, err
	}

	out := make([]RSRQItem, 0, n)

	for i := 0; i < n; i++ {
		extPresent, opt, err := r.ReadSequencePreamble(true, 2)
		if err != nil {
			return nil, err
		}

		var it RSRQItem

		if it.PCI, err = readExtConstrainedInt(r, 0, 503); err != nil {
			return nil, err
		}

		if it.EARFCN, err = readExtConstrainedInt(r, 0, 65535); err != nil {
			return nil, err
		}

		if opt[0] {
			ecgi, err := decodeECGI(r)
			if err != nil {
				return nil, err
			}

			it.ECGI = &ecgi
		}

		if it.ValueRSRQ, err = readExtConstrainedInt(r, 0, 34); err != nil {
			return nil, err
		}

		if err := skipSequenceTail(r, opt[1], extPresent); err != nil {
			return nil, err
		}

		out = append(out, it)
	}

	return out, nil
}

func parseFailure(value []byte) (*ECIDFailure, error) {
	fields, err := decodeMessageIEs(value)
	if err != nil {
		return nil, err
	}

	fail := &ECIDFailure{}

	var seenID, seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idESMLCUEMeasurementID:
			fail.ESMLCUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenID = true
		case idCause:
			fail.Cause, err = decodeCause(sub)
			seenCause = true
		}

		if err != nil {
			return nil, fmt.Errorf("lppa: failure IE %d: %w", f.id, err)
		}
	}

	if !seenID || !seenCause {
		return nil, fmt.Errorf("lppa: E-CID failure missing mandatory IE")
	}

	return fail, nil
}

func parseFailureIndication(value []byte) (*ECIDFailureIndication, error) {
	fields, err := decodeMessageIEs(value)
	if err != nil {
		return nil, err
	}

	fi := &ECIDFailureIndication{}

	var seenESMLC, seenENB, seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idESMLCUEMeasurementID:
			fi.ESMLCUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenESMLC = true
		case idENBUEMeasurementID:
			fi.ENBUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenENB = true
		case idCause:
			fi.Cause, err = decodeCause(sub)
			seenCause = true
		}

		if err != nil {
			return nil, fmt.Errorf("lppa: failure-indication IE %d: %w", f.id, err)
		}
	}

	if !seenESMLC || !seenENB || !seenCause {
		return nil, fmt.Errorf("lppa: E-CID failure indication missing mandatory IE")
	}

	return fi, nil
}

func parseTermination(value []byte) (*ECIDTermination, error) {
	fields, err := decodeMessageIEs(value)
	if err != nil {
		return nil, err
	}

	term := &ECIDTermination{}

	var seenESMLC, seenENB bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idESMLCUEMeasurementID:
			term.ESMLCUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenESMLC = true
		case idENBUEMeasurementID:
			term.ENBUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenENB = true
		}

		if err != nil {
			return nil, fmt.Errorf("lppa: termination IE %d: %w", f.id, err)
		}
	}

	if !seenESMLC || !seenENB {
		return nil, fmt.Errorf("lppa: E-CID termination missing mandatory IE")
	}

	return term, nil
}

func decodeCause(r *aper.Reader) (Cause, error) {
	grp, isExt, err := r.ReadChoiceIndex(causeRootCount, true)
	if err != nil {
		return Cause{}, err
	}

	if isExt {
		if _, err := r.ReadOpenType(); err != nil {
			return Cause{}, err
		}

		return Cause{Group: CauseGroupChoiceExtension}, nil
	}

	group := CauseGroup(grp)

	val, _, err := r.ReadEnum(causeGroupNRoot(group), true)
	if err != nil {
		return Cause{}, err
	}

	return Cause{Group: group, Value: int64(val)}, nil
}

// skipSequenceTail steps over a SEQUENCE's optional iE-Extensions container (when
// present) and any extension additions (when present) that this codec does not
// model.
func skipSequenceTail(r *aper.Reader, extContainer, extAdditions bool) error {
	if extContainer {
		if err := skipExtensionContainer(r); err != nil {
			return err
		}
	}

	if extAdditions {
		return r.SkipExtensionAdditions()
	}

	return nil
}

// skipExtensionContainer consumes a ProtocolExtensionContainer and discards it
// (TS 36.455 §9.3.4).
func skipExtensionContainer(r *aper.Reader) error {
	n, err := r.ReadConstrainedLength(1, maxProtocolExtensions)
	if err != nil {
		return fmt.Errorf("lppa: extension container length: %w", err)
	}

	for i := 0; i < n; i++ {
		if _, err := r.ReadConstrainedInt(0, maxProtocolIEs); err != nil {
			return err
		}

		if _, _, err := r.ReadEnum(criticalityRootCount, false); err != nil {
			return err
		}

		if _, err := r.ReadOpenType(); err != nil {
			return err
		}
	}

	return nil
}

// bitsToUint reads the first nbits of b, most significant bit first.
func bitsToUint(b []byte, nbits int) uint64 {
	var v uint64

	for i := 0; i < nbits; i++ {
		byteIdx := i / 8
		if byteIdx >= len(b) {
			break
		}

		if b[byteIdx]&(1<<uint(7-i%8)) != 0 {
			v |= 1 << uint(nbits-1-i)
		}
	}

	return v
}
