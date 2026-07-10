// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// reportOnDemand and reportPeriodic are the ReportCharacteristics root values
// (TS 36.455 §9.2.4).
const (
	reportOnDemand = 0
	reportPeriodic = 1

	reportCharacteristicsRootCount = 2
	measurementQuantityRootCount   = 6
)

// BuildECIDMeasurementInitiationRequest encodes an on-demand E-CID Measurement
// Initiation Request for the given quantities (TS 36.455 §8.2.1). esmlcMeasID is
// the E-SMLC-UE-Measurement-ID the eNB echoes for correlation.
func BuildECIDMeasurementInitiationRequest(esmlcMeasID int64, quantities []MeasurementQuantityValue) ([]byte, error) {
	if err := validateMeasurementID("esmlcMeasID", esmlcMeasID); err != nil {
		return nil, err
	}

	if len(quantities) < 1 || len(quantities) > maxNoMeas {
		return nil, fmt.Errorf("lppa: quantities length %d outside [1, %d]", len(quantities), maxNoMeas)
	}

	for _, q := range quantities {
		if q < MeasCellID || q > MeasRSRQ {
			return nil, fmt.Errorf("lppa: measurement quantity %d out of range [0, %d]", q, MeasRSRQ)
		}
	}

	fields := []ieField{
		{id: idESMLCUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(esmlcMeasID)},
		{id: idReportCharacteristics, crit: CriticalityReject, enc: encReportCharacteristics(reportOnDemand)},
		{id: idMeasurementQuantities, crit: CriticalityReject, enc: encMeasurementQuantities(quantities)},
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		return nil, err
	}

	return marshalPDU(pduInitiatingMessage, ProcECIDMeasurementInitiation, body)
}

// BuildECIDMeasurementTerminationCommand encodes an E-CID Measurement
// Termination Command releasing the measurement association in the eNB
// (TS 36.455 §8.2.4).
func BuildECIDMeasurementTerminationCommand(esmlcMeasID, enbMeasID int64) ([]byte, error) {
	if err := validateMeasurementID("esmlcMeasID", esmlcMeasID); err != nil {
		return nil, err
	}

	if err := validateMeasurementID("enbMeasID", enbMeasID); err != nil {
		return nil, err
	}

	fields := []ieField{
		{id: idESMLCUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(esmlcMeasID)},
		{id: idENBUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(enbMeasID)},
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		return nil, err
	}

	return marshalPDU(pduInitiatingMessage, ProcECIDMeasurementTermination, body)
}

// BuildECIDMeasurementInitiationResponse encodes an E-CID Measurement Initiation
// Response (TS 36.455 §8.2.1). result is optional.
func BuildECIDMeasurementInitiationResponse(esmlcMeasID, enbMeasID int64, result *ECIDResult) ([]byte, error) {
	if err := validateMeasurementID("esmlcMeasID", esmlcMeasID); err != nil {
		return nil, err
	}

	if err := validateMeasurementID("enbMeasID", enbMeasID); err != nil {
		return nil, err
	}

	fields := []ieField{
		{id: idESMLCUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(esmlcMeasID)},
		{id: idENBUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(enbMeasID)},
	}

	if result != nil {
		fields = append(fields, ieField{id: idECIDMeasurementResult, crit: CriticalityIgnore, enc: encMeasurementResult(result)})
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		return nil, err
	}

	return marshalPDU(pduSuccessfulOutcome, ProcECIDMeasurementInitiation, body)
}

// BuildECIDMeasurementInitiationFailure encodes an E-CID Measurement Initiation
// Failure carrying the rejection cause (TS 36.455 §8.2.1).
func BuildECIDMeasurementInitiationFailure(esmlcMeasID int64, cause Cause) ([]byte, error) {
	if err := validateMeasurementID("esmlcMeasID", esmlcMeasID); err != nil {
		return nil, err
	}

	fields := []ieField{
		{id: idESMLCUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(esmlcMeasID)},
		{id: idCause, crit: CriticalityIgnore, enc: encCause(cause)},
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		return nil, err
	}

	return marshalPDU(pduUnsuccessfulOutcome, ProcECIDMeasurementInitiation, body)
}

// validateMeasurementID rejects a Measurement-ID outside its root range 1..15
// (TS 36.455 §9.2.6) with an argument-named error.
func validateMeasurementID(name string, id int64) error {
	if id < 1 || id > 15 {
		return fmt.Errorf("lppa: %s %d out of range [1, 15]", name, id)
	}

	return nil
}

// encodeMessageBody writes an E-CID message SEQUENCE: an extensible preamble
// with no optional root fields, then the ProtocolIE-Container.
func encodeMessageBody(fields []ieField) ([]byte, error) {
	var w aper.Writer

	w.WriteSequencePreamble(true, false, nil)

	if err := encodeIEContainer(&w, fields); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// encMeasurementID encodes a Measurement-ID ::= INTEGER (1..15, ...)
// (TS 36.455 §9.2.6).
func encMeasurementID(id int64) func(*aper.Writer) error {
	return func(w *aper.Writer) error {
		return writeExtConstrainedInt(w, id, 1, 15)
	}
}

func encReportCharacteristics(v int) func(*aper.Writer) error {
	return func(w *aper.Writer) error {
		return w.WriteEnum(v, reportCharacteristicsRootCount, true, false)
	}
}

// encMeasurementQuantities encodes MeasurementQuantities ::= SEQUENCE (SIZE
// (1..maxNoMeas)) OF ProtocolIE-Single-Container (TS 36.455 §9.2.29).
func encMeasurementQuantities(qs []MeasurementQuantityValue) func(*aper.Writer) error {
	return func(w *aper.Writer) error {
		if len(qs) < 1 || len(qs) > maxNoMeas {
			return fmt.Errorf("lppa: measurement quantities length %d outside [1, %d]", len(qs), maxNoMeas)
		}

		if err := w.WriteConstrainedLength(len(qs), 1, maxNoMeas); err != nil {
			return err
		}

		for _, q := range qs {
			if err := w.WriteConstrainedInt(int64(idMeasurementQuantitiesItem), 0, maxProtocolIEs); err != nil {
				return err
			}

			if err := w.WriteEnum(int(CriticalityReject), criticalityRootCount, false, false); err != nil {
				return err
			}

			var vw aper.Writer

			vw.WriteSequencePreamble(true, false, []bool{false})

			if err := vw.WriteEnum(int(q), measurementQuantityRootCount, true, false); err != nil {
				return err
			}

			if err := w.WriteOpenType(vw.Bytes()); err != nil {
				return err
			}
		}

		return nil
	}
}

// encMeasurementResult encodes E-CID-MeasurementResult (TS 36.455 §9.2.5).
func encMeasurementResult(res *ECIDResult) func(*aper.Writer) error {
	return func(w *aper.Writer) error {
		hasAP := res.APPosition != nil
		hasMeasured := res.AngleOfArrival != nil || res.TimingAdvanceType1 != nil ||
			res.TimingAdvanceType2 != nil || len(res.RSRP) > 0 || len(res.RSRQ) > 0

		w.WriteSequencePreamble(true, false, []bool{hasAP, hasMeasured})

		if err := encECGI(w, res.ServingCell); err != nil {
			return err
		}

		if err := w.WriteOctetString(res.ServingCellTAC, 2, 2, false); err != nil {
			return err
		}

		if hasAP {
			if err := encAPPosition(w, res.APPosition); err != nil {
				return err
			}
		}

		if hasMeasured {
			if err := encMeasuredResults(w, res); err != nil {
				return err
			}
		}

		return nil
	}
}

// encECGI encodes ECGI ::= SEQUENCE { pLMN-Identity, eUTRANcellIdentifier
// BIT STRING (SIZE (28)), ... } (TS 36.455 §9.2.9).
func encECGI(w *aper.Writer, e ECGI) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := w.WriteOctetString(e.PLMNIdentity, 3, 3, false); err != nil {
		return err
	}

	return w.WriteBitString(uintToBits(e.EUTRACellID, 28), 28, 28, 28, false)
}

// encAPPosition encodes E-UTRANAccessPointPosition (TS 36.455 §9.2.1). The
// SEQUENCE is extensible with no optional root fields.
func encAPPosition(w *aper.Writer, p *APPosition) error {
	w.WriteSequencePreamble(true, false, nil)

	if err := w.WriteEnum(p.LatitudeSign, 2, false, false); err != nil {
		return err
	}

	for _, f := range []struct {
		v      int64
		lb, ub int64
	}{
		{p.Latitude, 0, 8388607},
		{p.Longitude, -8388608, 8388607},
	} {
		if err := w.WriteConstrainedInt(f.v, f.lb, f.ub); err != nil {
			return err
		}
	}

	if err := w.WriteEnum(p.DirectionOfAltitude, 2, false, false); err != nil {
		return err
	}

	for _, f := range []struct {
		v      int64
		lb, ub int64
	}{
		{p.Altitude, 0, 32767},
		{p.UncertaintySemiMajor, 0, 127},
		{p.UncertaintySemiMinor, 0, 127},
		{p.OrientationOfMajorAxis, 0, 179},
		{p.UncertaintyAltitude, 0, 127},
		{p.Confidence, 0, 100},
	} {
		if err := w.WriteConstrainedInt(f.v, f.lb, f.ub); err != nil {
			return err
		}
	}

	return nil
}

// encMeasuredResults encodes MeasuredResults ::= SEQUENCE (SIZE (1..maxNoMeas))
// OF MeasuredResultsValue, one CHOICE entry per present quantity
// (TS 36.455 §9.2.28).
func encMeasuredResults(w *aper.Writer, res *ECIDResult) error {
	var entries []func(*aper.Writer) error

	if res.AngleOfArrival != nil {
		v := *res.AngleOfArrival

		entries = append(entries, func(w *aper.Writer) error {
			return encMeasuredChoiceInt(w, 0, v, 0, 719)
		})
	}

	if res.TimingAdvanceType1 != nil {
		v := *res.TimingAdvanceType1

		entries = append(entries, func(w *aper.Writer) error {
			return encMeasuredChoiceInt(w, 1, v, 0, 7690)
		})
	}

	if res.TimingAdvanceType2 != nil {
		v := *res.TimingAdvanceType2

		entries = append(entries, func(w *aper.Writer) error {
			return encMeasuredChoiceInt(w, 2, v, 0, 7690)
		})
	}

	if len(res.RSRP) > 0 {
		items := res.RSRP

		entries = append(entries, func(w *aper.Writer) error {
			return encMeasuredChoiceList(w, 3, func(w *aper.Writer) error { return encResultRSRP(w, items) })
		})
	}

	if len(res.RSRQ) > 0 {
		items := res.RSRQ

		entries = append(entries, func(w *aper.Writer) error {
			return encMeasuredChoiceList(w, 4, func(w *aper.Writer) error { return encResultRSRQ(w, items) })
		})
	}

	if len(entries) < 1 || len(entries) > maxNoMeas {
		return fmt.Errorf("lppa: measured results length %d outside [1, %d]", len(entries), maxNoMeas)
	}

	if err := w.WriteConstrainedLength(len(entries), 1, maxNoMeas); err != nil {
		return err
	}

	for _, enc := range entries {
		if err := enc(w); err != nil {
			return err
		}
	}

	return nil
}

const measuredResultsRootCount = 5

// encMeasuredChoiceInt writes a MeasuredResultsValue CHOICE whose alternative is
// a constrained INTEGER (valueAngleOfArrival, valueTimingAdvanceType1/2).
func encMeasuredChoiceInt(w *aper.Writer, index int, v, lb, ub int64) error {
	if err := w.WriteChoiceIndex(index, measuredResultsRootCount, true, false); err != nil {
		return err
	}

	return w.WriteConstrainedInt(v, lb, ub)
}

// encMeasuredChoiceList writes a MeasuredResultsValue CHOICE whose alternative is
// a SEQUENCE-OF list (resultRSRP, resultRSRQ).
func encMeasuredChoiceList(w *aper.Writer, index int, enc func(*aper.Writer) error) error {
	if err := w.WriteChoiceIndex(index, measuredResultsRootCount, true, false); err != nil {
		return err
	}

	return enc(w)
}

// encResultRSRP encodes ResultRSRP ::= SEQUENCE (SIZE (1..maxCellReport)) OF
// ResultRSRP-Item (TS 36.455 §9.2.36).
func encResultRSRP(w *aper.Writer, items []RSRPItem) error {
	if len(items) < 1 || len(items) > maxCellReport {
		return fmt.Errorf("lppa: RSRP items length %d outside [1, %d]", len(items), maxCellReport)
	}

	if err := w.WriteConstrainedLength(len(items), 1, maxCellReport); err != nil {
		return err
	}

	for _, it := range items {
		hasECGI := it.ECGI != nil
		w.WriteSequencePreamble(true, false, []bool{hasECGI, false})

		if err := writeExtConstrainedInt(w, it.PCI, 0, 503); err != nil {
			return err
		}

		if err := writeExtConstrainedInt(w, it.EARFCN, 0, 65535); err != nil {
			return err
		}

		if hasECGI {
			if err := encECGI(w, *it.ECGI); err != nil {
				return err
			}
		}

		if err := writeExtConstrainedInt(w, it.ValueRSRP, 0, 97); err != nil {
			return err
		}
	}

	return nil
}

// encResultRSRQ encodes ResultRSRQ ::= SEQUENCE (SIZE (1..maxCellReport)) OF
// ResultRSRQ-Item (TS 36.455 §9.2.37).
func encResultRSRQ(w *aper.Writer, items []RSRQItem) error {
	if len(items) < 1 || len(items) > maxCellReport {
		return fmt.Errorf("lppa: RSRQ items length %d outside [1, %d]", len(items), maxCellReport)
	}

	if err := w.WriteConstrainedLength(len(items), 1, maxCellReport); err != nil {
		return err
	}

	for _, it := range items {
		hasECGI := it.ECGI != nil
		w.WriteSequencePreamble(true, false, []bool{hasECGI, false})

		if err := writeExtConstrainedInt(w, it.PCI, 0, 503); err != nil {
			return err
		}

		if err := writeExtConstrainedInt(w, it.EARFCN, 0, 65535); err != nil {
			return err
		}

		if hasECGI {
			if err := encECGI(w, *it.ECGI); err != nil {
				return err
			}
		}

		if err := writeExtConstrainedInt(w, it.ValueRSRQ, 0, 34); err != nil {
			return err
		}
	}

	return nil
}

// encCause encodes Cause ::= CHOICE { radioNetwork, protocol, misc, ... }
// (TS 36.455 §9.2.2). Only the three root ENUMERATED groups are emitted.
func encCause(c Cause) func(*aper.Writer) error {
	return func(w *aper.Writer) error {
		if c.Group < CauseGroupRadioNetwork || c.Group > CauseGroupMisc {
			return fmt.Errorf("lppa: cannot encode cause group %d", c.Group)
		}

		if err := w.WriteChoiceIndex(int(c.Group), causeRootCount, true, false); err != nil {
			return err
		}

		// Each root Cause group is an extensible ENUMERATED; the ordinal count is
		// not modeled, so the value is emitted as a root ENUMERATED index.
		return w.WriteEnum(int(c.Value), causeGroupNRoot(c.Group), true, false)
	}
}

const causeRootCount = 3

// causeGroupNRoot returns the number of root ENUMERATED values for a Cause group
// (TS 36.455 §9.2.2): CauseRadioNetwork has 3, CauseProtocol 7, CauseMisc 1.
func causeGroupNRoot(g CauseGroup) int {
	switch g {
	case CauseGroupRadioNetwork:
		return 3
	case CauseGroupProtocol:
		return 7
	case CauseGroupMisc:
		return 1
	default:
		return 1
	}
}

// uintToBits packs v into ceil(nbits/8) octets, most significant bit first.
func uintToBits(v uint64, nbits int) []byte {
	out := make([]byte, (nbits+7)/8)

	for i := 0; i < nbits; i++ {
		if v&(1<<uint(nbits-1-i)) != 0 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}

	return out
}
