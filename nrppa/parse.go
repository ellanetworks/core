// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

const maxProtocolExtensions = 65535

// ParsePDU decodes an NRPPA-PDU. An unrecognised procedure yields KindUnknown
// and no error.
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

func decodeMessageIEs(value []byte) ([]rawIE, error) {
	r := per.NewReader(value)

	extPresent, _, err := readSeqPreamble(r, 0)
	if err != nil {
		return nil, fmt.Errorf("nrppa: message preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipExtensionAdditions(r); err != nil {
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
		sub := per.NewReader(f.value)

		switch f.id {
		case idLMFUEMeasurementID:
			req.LMFUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenID = true
		case idReportCharacteristics:
			var idx int

			idx, err = decodeEnumInt(sub, reportCharacteristicsRootCount, true)
			req.ReportCharacteristics = idx
			seenReport = true
		case idMeasurementQuantities:
			req.MeasurementQuantities, err = decodeMeasurementQuantities(sub)
			seenQuantities = true
		}

		if err != nil {
			return nil, fmt.Errorf("nrppa: request IE %d: %w", f.id, err)
		}
	}

	if !seenID || !seenReport || !seenQuantities {
		return nil, fmt.Errorf("nrppa: E-CID request missing mandatory IE")
	}

	return req, nil
}

func decodeMeasurementQuantities(r *per.Reader) ([]MeasurementQuantityValue, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 1, maxNoMeas)
	if err != nil {
		return nil, err
	}

	out := make([]MeasurementQuantityValue, 0, n)

	for i := int64(0); i < n; i++ {
		if _, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs); err != nil {
			return nil, err
		}

		if _, err := per.DecodeEnumerated(r, per.Aligned, int64(criticalityRootCount), false); err != nil {
			return nil, err
		}

		item, err := per.DecodeOpenTypeBytes(r, per.Aligned)
		if err != nil {
			return nil, err
		}

		ir := per.NewReader(item)

		extPresent, opt, err := readSeqPreamble(ir, 1)
		if err != nil {
			return nil, err
		}

		idx, err := decodeEnumInt(ir, measurementQuantityRootCount, true)
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

	var seenLMF, seenRAN bool

	for _, f := range fields {
		sub := per.NewReader(f.value)

		switch f.id {
		case idLMFUEMeasurementID:
			resp.LMFUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenLMF = true
		case idRANUEMeasurementID:
			resp.RANUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenRAN = true
		case idECIDMeasurementResult:
			resp.Result, err = decodeMeasurementResult(sub)
		case idCellPortionID:
			var v int64

			v, err = readExtConstrainedInt(sub, 0, 4095)
			resp.CellPortionID = &v
		}

		if err != nil {
			return nil, fmt.Errorf("nrppa: response IE %d: %w", f.id, err)
		}
	}

	if !seenLMF || !seenRAN {
		return nil, fmt.Errorf("nrppa: E-CID response missing mandatory IE")
	}

	return resp, nil
}

func decodeMeasurementResult(r *per.Reader) (*ECIDResult, error) {
	extPresent, opt, err := readSeqPreamble(r, 3)
	if err != nil {
		return nil, err
	}

	res := &ECIDResult{}

	res.ServingCell, err = decodeNGRANCGI(r)
	if err != nil {
		return nil, err
	}

	res.ServingCellTAC, err = per.DecodeOctetString(r, per.Aligned, 3, 3, true, true, false)
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

	if err := skipSequenceTail(r, opt[2], extPresent); err != nil {
		return nil, err
	}

	return res, nil
}

func decodeNGRANCGI(r *per.Reader) (NGRANCGI, error) {
	extPresent, opt, err := readSeqPreamble(r, 1)
	if err != nil {
		return NGRANCGI{}, err
	}

	var out NGRANCGI

	out.PLMNIdentity, err = per.DecodeOctetString(r, per.Aligned, 3, 3, true, true, false)
	if err != nil {
		return NGRANCGI{}, err
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, ngRANCellAlternatives-1)
	if err != nil {
		return NGRANCGI{}, err
	}

	switch idx {
	case ngRANCellChoiceExtension:
		// An identity this codec does not model: it is unusable, but the rest of
		// the result still decodes.
		if _, err := per.DecodeOpenTypeBytes(r, per.Aligned); err != nil {
			return NGRANCGI{}, err
		}
	case ngRANCellEUTRACellID:
		bits, _, err := per.DecodeBitString(r, per.Aligned, 28, 28, true, true, false)
		if err != nil {
			return NGRANCGI{}, err
		}

		v := bitsToUint(bits, 28)
		out.EUTRACellID = &v
	case ngRANCellNRCellID:
		bits, _, err := per.DecodeBitString(r, per.Aligned, 36, 36, true, true, false)
		if err != nil {
			return NGRANCGI{}, err
		}

		v := bitsToUint(bits, 36)
		out.NRCellIdentity = &v
	}

	if err := skipSequenceTail(r, opt[0], extPresent); err != nil {
		return NGRANCGI{}, err
	}

	return out, nil
}

func decodeCGIEUTRA(r *per.Reader) (*CGIEUTRA, error) {
	extPresent, opt, err := readSeqPreamble(r, 1)
	if err != nil {
		return nil, err
	}

	plmn, err := per.DecodeOctetString(r, per.Aligned, 3, 3, true, true, false)
	if err != nil {
		return nil, err
	}

	bits, _, err := per.DecodeBitString(r, per.Aligned, 28, 28, true, true, false)
	if err != nil {
		return nil, err
	}

	if err := skipSequenceTail(r, opt[0], extPresent); err != nil {
		return nil, err
	}

	return &CGIEUTRA{PLMNIdentity: plmn, EUTRACellID: bitsToUint(bits, 28)}, nil
}

func decodeCGINR(r *per.Reader) (*CGINR, error) {
	extPresent, opt, err := readSeqPreamble(r, 1)
	if err != nil {
		return nil, err
	}

	plmn, err := per.DecodeOctetString(r, per.Aligned, 3, 3, true, true, false)
	if err != nil {
		return nil, err
	}

	bits, _, err := per.DecodeBitString(r, per.Aligned, 36, 36, true, true, false)
	if err != nil {
		return nil, err
	}

	if err := skipSequenceTail(r, opt[0], extPresent); err != nil {
		return nil, err
	}

	return &CGINR{PLMNIdentity: plmn, NRCellIdentity: bitsToUint(bits, 36)}, nil
}

func decodeAPPosition(r *per.Reader) (*APPosition, error) {
	extPresent, _, err := readSeqPreamble(r, 0)
	if err != nil {
		return nil, err
	}

	p := &APPosition{}

	sign, err := decodeEnumInt(r, 2, false)
	if err != nil {
		return nil, err
	}

	p.LatitudeSign = sign

	p.Latitude, err = per.DecodeInteger(r, per.Aligned, per.Bounds{LB: 0, HasLB: true, UB: 8388607, HasUB: true})
	if err != nil {
		return nil, err
	}

	p.Longitude, err = per.DecodeInteger(r, per.Aligned, per.Bounds{LB: -8388608, HasLB: true, UB: 8388607, HasUB: true})
	if err != nil {
		return nil, err
	}

	dir, err := decodeEnumInt(r, 2, false)
	if err != nil {
		return nil, err
	}

	p.DirectionOfAltitude = dir

	for _, f := range []struct {
		dst    *int64
		lb, ub int64
	}{
		{&p.Altitude, 0, 32767},
		{&p.UncertaintySemiMajor, 0, 127},
		{&p.UncertaintySemiMinor, 0, 127},
		{&p.OrientationOfMajorAxis, 0, 179},
		{&p.UncertaintyAltitude, 0, 127},
		{&p.Confidence, 0, 100},
	} {
		*f.dst, err = per.DecodeConstrainedWholeNumber(r, per.Aligned, f.lb, f.ub)
		if err != nil {
			return nil, err
		}
	}

	if err := skipSequenceTail(r, false, extPresent); err != nil {
		return nil, err
	}

	p.LatitudeDegrees, p.LongitudeDegrees = apToDegrees(p)

	return p, nil
}

func apToDegrees(p *APPosition) (lat, lon float64) {
	lat = float64(p.Latitude) * 90.0 / 8388608.0
	if p.LatitudeSign == 1 {
		lat = -lat
	}

	lon = float64(p.Longitude) * 360.0 / 16777216.0

	return lat, lon
}

func decodeMeasuredResults(r *per.Reader, res *ECIDResult) error {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 1, maxNoMeas)
	if err != nil {
		return err
	}

	for range n {
		idx, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, measuredResultsAlternatives-1)
		if err != nil {
			return err
		}

		switch idx {
		case measuredAngleOfArrivalEUTRA:
			v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 719)
			if err != nil {
				return err
			}

			res.AngleOfArrival = &v
		case measuredTimingAdvanceType1EUTRA:
			v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 7690)
			if err != nil {
				return err
			}

			res.TimingAdvanceType1 = &v
		case measuredTimingAdvanceType2EUTRA:
			v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 7690)
			if err != nil {
				return err
			}

			res.TimingAdvanceType2 = &v
		case measuredResultRSRPEUTRA:
			res.RSRP, err = decodeResultRSRP(r)
			if err != nil {
				return err
			}
		case measuredResultRSRQEUTRA:
			res.RSRQ, err = decodeResultRSRQ(r)
			if err != nil {
				return err
			}
		case measuredChoiceExtension:
			if err := decodeMeasuredChoiceExtension(r, res); err != nil {
				return err
			}
		}
	}

	return nil
}

// decodeMeasuredChoiceExtension reads the ProtocolIE-Single-Container that
// carries every NR quantity (TS 38.455 §9.2.27). An id this codec does not
// model is skipped with its open type, leaving the rest of the list readable.
func decodeMeasuredChoiceExtension(r *per.Reader, res *ECIDResult) error {
	id, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs)
	if err != nil {
		return err
	}

	if _, err := per.DecodeEnumerated(r, per.Aligned, int64(criticalityRootCount), false); err != nil {
		return err
	}

	value, err := per.DecodeOpenTypeBytes(r, per.Aligned)
	if err != nil {
		return err
	}

	sub := per.NewReader(value)

	switch ProtocolIEID(id) {
	case idResultSSRSRP:
		res.SSRSRP, err = decodeSSRSRP(sub)
	case idResultSSRSRQ:
		res.SSRSRQ, err = decodeSSRSRQ(sub)
	case idResultCSIRSRP:
		res.CSIRSRP, err = decodeCSIRSRP(sub)
	case idResultCSIRSRQ:
		res.CSIRSRQ, err = decodeCSIRSRQ(sub)
	case idAngleOfArrivalNR:
		res.AoA, err = decodeULAoA(sub)
	case idNRTADV:
		var v int64

		v, err = per.DecodeConstrainedWholeNumber(sub, per.Aligned, 0, 7690)
		res.NRTimingAdvance = &v
	case idUERxTxTimeDiff:
		var v int64

		v, err = per.DecodeConstrainedWholeNumber(sub, per.Aligned, 0, 61565)
		res.UERxTxTimeDiff = &v
	}

	if err != nil {
		return fmt.Errorf("nrppa: measured result IE %d: %w", id, err)
	}

	return nil
}

// nrCellHeader is the nR-PCI / nR-ARFCN / cGI-NR prefix the four NR per-cell
// result items share, plus the presence bits of the two fields that follow.
type nrCellHeader struct {
	pci      int64
	arfcn    int64
	cgi      *CGINR
	hasValue bool
	hasPer   bool
	extSeq   bool
	extCont  bool
}

func decodeNRCellHeader(r *per.Reader) (nrCellHeader, error) {
	extPresent, opt, err := readSeqPreamble(r, 4)
	if err != nil {
		return nrCellHeader{}, err
	}

	h := nrCellHeader{hasValue: opt[1], hasPer: opt[2], extSeq: extPresent, extCont: opt[3]}

	h.pci, err = per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 1007)
	if err != nil {
		return nrCellHeader{}, err
	}

	h.arfcn, err = per.DecodeInteger(r, per.Aligned, nrARFCNBounds)
	if err != nil {
		return nrCellHeader{}, err
	}

	if opt[0] {
		h.cgi, err = decodeCGINR(r)
		if err != nil {
			return nrCellHeader{}, err
		}
	}

	return h, nil
}

// decodeNRResultList reads the SEQUENCE (SIZE (1..maxCellReportNR)) OF wrapper
// the four NR per-cell result lists share.
func decodeNRResultList(r *per.Reader, decodeItem func(*per.Reader) error) error {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 1, maxCellReportNR)
	if err != nil {
		return err
	}

	for range n {
		if err := decodeItem(r); err != nil {
			return err
		}
	}

	return nil
}

func decodeSSRSRP(r *per.Reader) ([]SSRSRPItem, error) {
	var out []SSRSRPItem

	err := decodeNRResultList(r, func(r *per.Reader) error {
		h, err := decodeNRCellHeader(r)
		if err != nil {
			return err
		}

		it := SSRSRPItem{NRPCI: h.pci, NRARFCN: h.arfcn, CGI: h.cgi}

		if h.hasValue {
			v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 127)
			if err != nil {
				return err
			}

			it.Value = &v
		}

		if h.hasPer {
			it.PerSSB, err = decodePerSSB(r)
			if err != nil {
				return err
			}
		}

		if err := skipSequenceTail(r, h.extCont, h.extSeq); err != nil {
			return err
		}

		out = append(out, it)

		return nil
	})

	return out, err
}

func decodeSSRSRQ(r *per.Reader) ([]SSRSRQItem, error) {
	var out []SSRSRQItem

	err := decodeNRResultList(r, func(r *per.Reader) error {
		h, err := decodeNRCellHeader(r)
		if err != nil {
			return err
		}

		it := SSRSRQItem{NRPCI: h.pci, NRARFCN: h.arfcn, CGI: h.cgi}

		if h.hasValue {
			v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 127)
			if err != nil {
				return err
			}

			it.Value = &v
		}

		if h.hasPer {
			it.PerSSB, err = decodePerSSB(r)
			if err != nil {
				return err
			}
		}

		if err := skipSequenceTail(r, h.extCont, h.extSeq); err != nil {
			return err
		}

		out = append(out, it)

		return nil
	})

	return out, err
}

func decodeCSIRSRP(r *per.Reader) ([]CSIRSRPItem, error) {
	var out []CSIRSRPItem

	err := decodeNRResultList(r, func(r *per.Reader) error {
		h, err := decodeNRCellHeader(r)
		if err != nil {
			return err
		}

		it := CSIRSRPItem{NRPCI: h.pci, NRARFCN: h.arfcn, CGI: h.cgi}

		if h.hasValue {
			v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 127)
			if err != nil {
				return err
			}

			it.Value = &v
		}

		if h.hasPer {
			it.PerCSIRS, err = decodePerCSIRS(r)
			if err != nil {
				return err
			}
		}

		if err := skipSequenceTail(r, h.extCont, h.extSeq); err != nil {
			return err
		}

		out = append(out, it)

		return nil
	})

	return out, err
}

func decodeCSIRSRQ(r *per.Reader) ([]CSIRSRQItem, error) {
	var out []CSIRSRQItem

	err := decodeNRResultList(r, func(r *per.Reader) error {
		h, err := decodeNRCellHeader(r)
		if err != nil {
			return err
		}

		it := CSIRSRQItem{NRPCI: h.pci, NRARFCN: h.arfcn, CGI: h.cgi}

		if h.hasValue {
			v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 127)
			if err != nil {
				return err
			}

			it.Value = &v
		}

		if h.hasPer {
			it.PerCSIRS, err = decodePerCSIRS(r)
			if err != nil {
				return err
			}
		}

		if err := skipSequenceTail(r, h.extCont, h.extSeq); err != nil {
			return err
		}

		out = append(out, it)

		return nil
	})

	return out, err
}

func decodePerSSB(r *per.Reader) ([]SSBResultItem, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 1, maxIndexesReport)
	if err != nil {
		return nil, err
	}

	out := make([]SSBResultItem, 0, n)

	for range n {
		extPresent, opt, err := readSeqPreamble(r, 1)
		if err != nil {
			return nil, err
		}

		idx, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 63)
		if err != nil {
			return nil, err
		}

		v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 127)
		if err != nil {
			return nil, err
		}

		if err := skipSequenceTail(r, opt[0], extPresent); err != nil {
			return nil, err
		}

		out = append(out, SSBResultItem{SSBIndex: idx, Value: v})
	}

	return out, nil
}

func decodePerCSIRS(r *per.Reader) ([]CSIRSResultItem, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 1, maxIndexesReport)
	if err != nil {
		return nil, err
	}

	out := make([]CSIRSResultItem, 0, n)

	for range n {
		extPresent, opt, err := readSeqPreamble(r, 1)
		if err != nil {
			return nil, err
		}

		idx, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 95)
		if err != nil {
			return nil, err
		}

		v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 127)
		if err != nil {
			return nil, err
		}

		if err := skipSequenceTail(r, opt[0], extPresent); err != nil {
			return nil, err
		}

		out = append(out, CSIRSResultItem{CSIRSIndex: idx, Value: v})
	}

	return out, nil
}

func decodeULAoA(r *per.Reader) (*AoAResult, error) {
	extPresent, opt, err := readSeqPreamble(r, 3)
	if err != nil {
		return nil, err
	}

	azimuth, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 3599)
	if err != nil {
		return nil, err
	}

	out := &AoAResult{AzimuthRaw: azimuth, AzimuthDegrees: float64(azimuth) / 10}

	if opt[0] {
		zenith, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 1799)
		if err != nil {
			return nil, err
		}

		deg := float64(zenith) / 10
		out.ZenithRaw = &zenith
		out.ZenithDegrees = &deg
	}

	if opt[1] {
		out.LCSToGCS, err = decodeLCSToGCS(r)
		if err != nil {
			return nil, err
		}
	}

	if err := skipSequenceTail(r, opt[2], extPresent); err != nil {
		return nil, err
	}

	return out, nil
}

func decodeLCSToGCS(r *per.Reader) (*LCSToGCS, error) {
	extPresent, opt, err := readSeqPreamble(r, 1)
	if err != nil {
		return nil, err
	}

	out := &LCSToGCS{}

	for _, dst := range []*float64{&out.AlphaDegrees, &out.BetaDegrees, &out.GammaDegrees} {
		v, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 3599)
		if err != nil {
			return nil, err
		}

		*dst = float64(v) / 10
	}

	if err := skipSequenceTail(r, opt[0], extPresent); err != nil {
		return nil, err
	}

	return out, nil
}

func decodeResultRSRP(r *per.Reader) ([]RSRPItem, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 1, maxCellReport)
	if err != nil {
		return nil, err
	}

	out := make([]RSRPItem, 0, n)

	for range n {
		extPresent, opt, err := readSeqPreamble(r, 2)
		if err != nil {
			return nil, err
		}

		var it RSRPItem

		it.PCI, err = per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 503)
		if err != nil {
			return nil, err
		}

		it.EARFCN, err = per.DecodeInteger(r, per.Aligned, earfcnBounds)
		if err != nil {
			return nil, err
		}

		if opt[0] {
			it.CGI, err = decodeCGIEUTRA(r)
			if err != nil {
				return nil, err
			}
		}

		it.ValueRSRP, err = per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 97)
		if err != nil {
			return nil, err
		}

		if err := skipSequenceTail(r, opt[1], extPresent); err != nil {
			return nil, err
		}

		out = append(out, it)
	}

	return out, nil
}

func decodeResultRSRQ(r *per.Reader) ([]RSRQItem, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 1, maxCellReport)
	if err != nil {
		return nil, err
	}

	out := make([]RSRQItem, 0, n)

	for range n {
		extPresent, opt, err := readSeqPreamble(r, 2)
		if err != nil {
			return nil, err
		}

		var it RSRQItem

		it.PCI, err = per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 503)
		if err != nil {
			return nil, err
		}

		it.EARFCN, err = per.DecodeInteger(r, per.Aligned, earfcnBounds)
		if err != nil {
			return nil, err
		}

		if opt[0] {
			it.CGI, err = decodeCGIEUTRA(r)
			if err != nil {
				return nil, err
			}
		}

		it.ValueRSRQ, err = per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 34)
		if err != nil {
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

	var seenLMF, seenCause bool

	for _, f := range fields {
		sub := per.NewReader(f.value)

		switch f.id {
		case idLMFUEMeasurementID:
			fail.LMFUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenLMF = true
		case idCause:
			fail.Cause, err = decodeCause(sub)
			seenCause = true
		}

		if err != nil {
			return nil, fmt.Errorf("nrppa: failure IE %d: %w", f.id, err)
		}
	}

	if !seenLMF || !seenCause {
		return nil, fmt.Errorf("nrppa: E-CID failure missing mandatory IE")
	}

	return fail, nil
}

func parseFailureIndication(value []byte) (*ECIDFailureIndication, error) {
	fields, err := decodeMessageIEs(value)
	if err != nil {
		return nil, err
	}

	ind := &ECIDFailureIndication{}

	var seenLMF, seenRAN, seenCause bool

	for _, f := range fields {
		sub := per.NewReader(f.value)

		switch f.id {
		case idLMFUEMeasurementID:
			ind.LMFUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenLMF = true
		case idRANUEMeasurementID:
			ind.RANUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenRAN = true
		case idCause:
			ind.Cause, err = decodeCause(sub)
			seenCause = true
		}

		if err != nil {
			return nil, fmt.Errorf("nrppa: failure indication IE %d: %w", f.id, err)
		}
	}

	if !seenLMF || !seenRAN || !seenCause {
		return nil, fmt.Errorf("nrppa: E-CID failure indication missing mandatory IE")
	}

	return ind, nil
}

func parseTermination(value []byte) (*ECIDTermination, error) {
	fields, err := decodeMessageIEs(value)
	if err != nil {
		return nil, err
	}

	term := &ECIDTermination{}

	var seenLMF, seenRAN bool

	for _, f := range fields {
		sub := per.NewReader(f.value)

		switch f.id {
		case idLMFUEMeasurementID:
			term.LMFUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenLMF = true
		case idRANUEMeasurementID:
			term.RANUEMeasurementID, err = readExtConstrainedInt(sub, 1, 15)
			seenRAN = true
		}

		if err != nil {
			return nil, fmt.Errorf("nrppa: termination IE %d: %w", f.id, err)
		}
	}

	if !seenLMF || !seenRAN {
		return nil, fmt.Errorf("nrppa: E-CID termination missing mandatory IE")
	}

	return term, nil
}

func decodeCause(r *per.Reader) (Cause, error) {
	grp, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, causeAlternatives-1)
	if err != nil {
		return Cause{}, err
	}

	if CauseGroup(grp) == CauseGroupChoiceExtension {
		if _, err := per.DecodeOpenTypeBytes(r, per.Aligned); err != nil {
			return Cause{}, err
		}

		return Cause{Group: CauseGroupChoiceExtension}, nil
	}

	group := CauseGroup(grp)

	val, err := decodeEnumInt(r, causeGroupNRoot(group), true)
	if err != nil {
		return Cause{}, err
	}

	return Cause{Group: group, Value: int64(val)}, nil
}

// skipSequenceTail steps over the unmodeled iE-Extensions container and
// extension additions at the end of a SEQUENCE.
func skipSequenceTail(r *per.Reader, extContainer, extAdditions bool) error {
	if extContainer {
		if err := skipExtensionContainer(r); err != nil {
			return err
		}
	}

	if extAdditions {
		return skipExtensionAdditions(r)
	}

	return nil
}

// skipExtensionContainer consumes a ProtocolExtensionContainer
// (TS 38.455 §9.3.4).
func skipExtensionContainer(r *per.Reader) error {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 1, maxProtocolExtensions)
	if err != nil {
		return fmt.Errorf("nrppa: extension container length: %w", err)
	}

	for i := int64(0); i < n; i++ {
		if _, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs); err != nil {
			return err
		}

		if _, err := per.DecodeEnumerated(r, per.Aligned, int64(criticalityRootCount), false); err != nil {
			return err
		}

		if _, err := per.DecodeOpenTypeBytes(r, per.Aligned); err != nil {
			return err
		}
	}

	return nil
}

// bitsToUint reads the first nbits of b, most significant bit first.
func bitsToUint(b []byte, nbits int) uint64 {
	var v uint64

	for i := range nbits {
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
