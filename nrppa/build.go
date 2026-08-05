// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// ReportCharacteristics root values (TS 38.455 §9.2.5).
const (
	reportOnDemand = 0
	reportPeriodic = 1

	reportCharacteristicsRootCount = 2

	// MeasurementQuantitiesValue has six root values; the NR quantities are
	// extension additions numbered from there (TS 38.455 §9.2.29).
	measurementQuantityRootCount = 6
	measurementQuantityMax       = MeasUERxTxTimeDiff
)

// BuildECIDMeasurementInitiationRequest encodes an on-demand request
// (TS 38.455 §8.2.1). lmfMeasID is echoed by the NG-RAN node for correlation.
func BuildECIDMeasurementInitiationRequest(lmfMeasID int64, quantities []MeasurementQuantityValue) ([]byte, error) {
	if err := validateMeasurementID("lmfMeasID", lmfMeasID); err != nil {
		return nil, err
	}

	if len(quantities) < 1 || len(quantities) > maxNoMeas {
		return nil, fmt.Errorf("nrppa: quantities length %d outside [1, %d]", len(quantities), maxNoMeas)
	}

	for _, q := range quantities {
		if q < MeasCellID || q > measurementQuantityMax {
			return nil, fmt.Errorf("nrppa: measurement quantity %d out of range [0, %d]", q, measurementQuantityMax)
		}
	}

	fields := []ieField{
		{id: idLMFUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(lmfMeasID)},
		{id: idReportCharacteristics, crit: CriticalityReject, enc: encReportCharacteristics(reportOnDemand)},
		{id: idMeasurementQuantities, crit: CriticalityReject, enc: encMeasurementQuantities(quantities)},
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		return nil, err
	}

	return marshalPDU(pduInitiatingMessage, ProcECIDMeasurementInitiation, body)
}

// BuildECIDMeasurementTerminationCommand releases the measurement association
// in the NG-RAN node (TS 38.455 §8.2.4).
func BuildECIDMeasurementTerminationCommand(lmfMeasID, ranMeasID int64) ([]byte, error) {
	if err := validateMeasurementID("lmfMeasID", lmfMeasID); err != nil {
		return nil, err
	}

	if err := validateMeasurementID("ranMeasID", ranMeasID); err != nil {
		return nil, err
	}

	fields := []ieField{
		{id: idLMFUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(lmfMeasID)},
		{id: idRANUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(ranMeasID)},
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		return nil, err
	}

	return marshalPDU(pduInitiatingMessage, ProcECIDMeasurementTermination, body)
}

// BuildECIDMeasurementInitiationResponse accepts a nil result (TS 38.455
// §8.2.1).
func BuildECIDMeasurementInitiationResponse(lmfMeasID, ranMeasID int64, result *ECIDResult) ([]byte, error) {
	if err := validateMeasurementID("lmfMeasID", lmfMeasID); err != nil {
		return nil, err
	}

	if err := validateMeasurementID("ranMeasID", ranMeasID); err != nil {
		return nil, err
	}

	fields := []ieField{
		{id: idLMFUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(lmfMeasID)},
		{id: idRANUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(ranMeasID)},
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

// BuildECIDMeasurementInitiationFailure encodes the rejection of an initiation
// request (TS 38.455 §8.2.1).
func BuildECIDMeasurementInitiationFailure(lmfMeasID int64, cause Cause) ([]byte, error) {
	if err := validateMeasurementID("lmfMeasID", lmfMeasID); err != nil {
		return nil, err
	}

	fields := []ieField{
		{id: idLMFUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(lmfMeasID)},
		{id: idCause, crit: CriticalityIgnore, enc: encCause(cause)},
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		return nil, err
	}

	return marshalPDU(pduUnsuccessfulOutcome, ProcECIDMeasurementInitiation, body)
}

// validateMeasurementID rejects values outside the UE-Measurement-ID root range
// 1..15 (TS 38.455 §9.2.6). The type extends to 256, but this codec only emits
// root values.
func validateMeasurementID(name string, id int64) error {
	if id < 1 || id > 15 {
		return fmt.Errorf("nrppa: %s %d out of range [1, 15]", name, id)
	}

	return nil
}

// encodeMessageBody writes an E-CID message SEQUENCE, which is extensible with
// no optional root field.
func encodeMessageBody(fields []ieField) ([]byte, error) {
	w := per.NewWriter()

	writeSeqPreamble(w, false, nil)

	if err := encodeIEContainer(w, fields); err != nil {
		return nil, err
	}

	return perAlignedBytes(w), nil
}

// UE-Measurement-ID ::= INTEGER (1..15, ..., 16..256) (TS 38.455 §9.2.6).
func encMeasurementID(id int64) func(*per.Writer) error {
	return func(w *per.Writer) error {
		return writeExtConstrainedInt(w, id, 1, 15)
	}
}

func encReportCharacteristics(v int) func(*per.Writer) error {
	return func(w *per.Writer) error {
		return per.EncodeEnumerated(w, per.Aligned, reportCharacteristicsRootCount, true, int64(v))
	}
}

// MeasurementQuantities ::= SEQUENCE (SIZE (1..maxNoMeas)) OF
// ProtocolIE-Single-Container (TS 38.455 §9.2.28).
func encMeasurementQuantities(qs []MeasurementQuantityValue) func(*per.Writer) error {
	return func(w *per.Writer) error {
		if len(qs) < 1 || len(qs) > maxNoMeas {
			return fmt.Errorf("nrppa: measurement quantities length %d outside [1, %d]", len(qs), maxNoMeas)
		}

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxNoMeas, int64(len(qs))); err != nil {
			return err
		}

		for _, q := range qs {
			if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(idMeasurementQuantitiesItem)); err != nil {
				return err
			}

			if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(int(CriticalityReject))); err != nil {
				return err
			}

			vw := per.NewWriter()

			writeSeqPreamble(vw, false, []bool{false})

			if err := per.EncodeEnumerated(vw, per.Aligned, measurementQuantityRootCount, true, int64(int(q))); err != nil {
				return err
			}

			if err := per.EncodeOpenTypeBytes(w, per.Aligned, perAlignedBytes(vw)); err != nil {
				return err
			}
		}

		return nil
	}
}

// E-CID-MeasurementResult (TS 38.455 §9.2.3).
func encMeasurementResult(res *ECIDResult) func(*per.Writer) error {
	return func(w *per.Writer) error {
		if len(res.ServingCellTAC) != 3 {
			return fmt.Errorf("nrppa: serving cell TAC must be 3 octets, got %d", len(res.ServingCellTAC))
		}

		hasAP := res.APPosition != nil
		hasMeasured := res.AngleOfArrival != nil || res.TimingAdvanceType1 != nil ||
			res.TimingAdvanceType2 != nil || len(res.RSRP) > 0 || len(res.RSRQ) > 0 ||
			len(res.SSRSRP) > 0 || len(res.SSRSRQ) > 0 || len(res.CSIRSRP) > 0 || len(res.CSIRSRQ) > 0 ||
			res.AoA != nil || res.NRTimingAdvance != nil || res.UERxTxTimeDiff != nil

		writeSeqPreamble(w, false, []bool{hasAP, hasMeasured, false})

		if err := encNGRANCGI(w, res.ServingCell); err != nil {
			return err
		}

		if err := per.EncodeOctetString(w, per.Aligned, 3, 3, true, true, false, res.ServingCellTAC); err != nil {
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

// NG-RANCell CHOICE alternatives (TS 38.455 §9.2.10). The CHOICE carries no
// extension marker: choice-Extension is a root alternative, so the index is a
// plain constrained number over all three. Every NRPPa CHOICE is built this
// way, where TS 36.455 marks its CHOICEs extensible instead.
const (
	ngRANCellEUTRACellID = iota
	ngRANCellNRCellID
	ngRANCellChoiceExtension

	ngRANCellAlternatives = 3
)

// NG-RAN-CGI (TS 38.455 §9.2.10). Where TS 36.455 §9.2.9 has a bare 28-bit
// E-UTRAN cell identity, NRPPa wraps the identity in a CHOICE so the same IE
// can name an NR or an E-UTRA cell.
func encNGRANCGI(w *per.Writer, c NGRANCGI) error {
	if len(c.PLMNIdentity) != 3 {
		return fmt.Errorf("nrppa: PLMN identity must be 3 octets, got %d", len(c.PLMNIdentity))
	}

	if (c.NRCellIdentity == nil) == (c.EUTRACellID == nil) {
		return fmt.Errorf("nrppa: serving cell must carry exactly one of an NR or an E-UTRA cell identity")
	}

	writeSeqPreamble(w, false, []bool{false})

	if err := per.EncodeOctetString(w, per.Aligned, 3, 3, true, true, false, c.PLMNIdentity); err != nil {
		return err
	}

	if c.NRCellIdentity != nil {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, ngRANCellAlternatives-1, ngRANCellNRCellID); err != nil {
			return err
		}

		return per.EncodeBitString(w, per.Aligned, 36, 36, true, true, false, uintToBits(*c.NRCellIdentity, 36), 36)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, ngRANCellAlternatives-1, ngRANCellEUTRACellID); err != nil {
		return err
	}

	return per.EncodeBitString(w, per.Aligned, 28, 28, true, true, false, uintToBits(*c.EUTRACellID, 28), 28)
}

// CGI-EUTRA (TS 38.455 §9.2.11), the optional per-item cell identity of an
// E-UTRA measurement result.
func encCGIEUTRA(w *per.Writer, c CGIEUTRA) error {
	if len(c.PLMNIdentity) != 3 {
		return fmt.Errorf("nrppa: PLMN identity must be 3 octets, got %d", len(c.PLMNIdentity))
	}

	writeSeqPreamble(w, false, []bool{false})

	if err := per.EncodeOctetString(w, per.Aligned, 3, 3, true, true, false, c.PLMNIdentity); err != nil {
		return err
	}

	return per.EncodeBitString(w, per.Aligned, 28, 28, true, true, false, uintToBits(c.EUTRACellID, 28), 28)
}

// CGI-NR (TS 38.455 §9.2.12), the optional per-item cell identity of an NR
// measurement result.
func encCGINR(w *per.Writer, c CGINR) error {
	if len(c.PLMNIdentity) != 3 {
		return fmt.Errorf("nrppa: PLMN identity must be 3 octets, got %d", len(c.PLMNIdentity))
	}

	writeSeqPreamble(w, false, []bool{false})

	if err := per.EncodeOctetString(w, per.Aligned, 3, 3, true, true, false, c.PLMNIdentity); err != nil {
		return err
	}

	return per.EncodeBitString(w, per.Aligned, 36, 36, true, true, false, uintToBits(c.NRCellIdentity, 36), 36)
}

// NG-RANAccessPointPosition (TS 38.455 §9.2.2). The ten position fields are
// those of the E-UTRAN position of TS 36.455 §9.2.1, but NRPPa adds an
// iE-Extensions field, so this SEQUENCE carries a presence bit LPPa's does not.
func encAPPosition(w *per.Writer, p *APPosition) error {
	writeSeqPreamble(w, false, []bool{false})

	if err := per.EncodeEnumerated(w, per.Aligned, 2, false, int64(p.LatitudeSign)); err != nil {
		return err
	}

	for _, f := range []struct {
		v      int64
		lb, ub int64
	}{
		{p.Latitude, 0, 8388607},
		{p.Longitude, -8388608, 8388607},
	} {
		if err := per.EncodeInteger(w, per.Aligned, per.Bounds{LB: f.lb, HasLB: true, UB: f.ub, HasUB: true}, f.v); err != nil {
			return err
		}
	}

	if err := per.EncodeEnumerated(w, per.Aligned, 2, false, int64(p.DirectionOfAltitude)); err != nil {
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
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, f.lb, f.ub, f.v); err != nil {
			return err
		}
	}

	return nil
}

// MeasuredResultsValue CHOICE alternatives (TS 38.455 §9.2.27). The five root
// alternatives are the E-UTRA ones LPPa also has; every NR quantity is carried
// through choice-Extension as a ProtocolIE-Single-Container.
const (
	measuredAngleOfArrivalEUTRA = iota
	measuredTimingAdvanceType1EUTRA
	measuredTimingAdvanceType2EUTRA
	measuredResultRSRPEUTRA
	measuredResultRSRQEUTRA
	measuredChoiceExtension

	measuredResultsAlternatives = 6
)

// MeasuredResults ::= SEQUENCE (SIZE (1..maxNoMeas)) OF MeasuredResultsValue
// (TS 38.455 §9.2.26), one CHOICE entry per quantity present in res.
func encMeasuredResults(w *per.Writer, res *ECIDResult) error {
	var entries []func(*per.Writer) error

	if res.AngleOfArrival != nil {
		v := *res.AngleOfArrival

		entries = append(entries, func(w *per.Writer) error {
			return encMeasuredChoiceInt(w, measuredAngleOfArrivalEUTRA, v, 0, 719)
		})
	}

	if res.TimingAdvanceType1 != nil {
		v := *res.TimingAdvanceType1

		entries = append(entries, func(w *per.Writer) error {
			return encMeasuredChoiceInt(w, measuredTimingAdvanceType1EUTRA, v, 0, 7690)
		})
	}

	if res.TimingAdvanceType2 != nil {
		v := *res.TimingAdvanceType2

		entries = append(entries, func(w *per.Writer) error {
			return encMeasuredChoiceInt(w, measuredTimingAdvanceType2EUTRA, v, 0, 7690)
		})
	}

	if len(res.RSRP) > 0 {
		items := res.RSRP

		entries = append(entries, func(w *per.Writer) error {
			return encMeasuredChoiceList(w, measuredResultRSRPEUTRA, func(w *per.Writer) error { return encResultRSRP(w, items) })
		})
	}

	if len(res.RSRQ) > 0 {
		items := res.RSRQ

		entries = append(entries, func(w *per.Writer) error {
			return encMeasuredChoiceList(w, measuredResultRSRQEUTRA, func(w *per.Writer) error { return encResultRSRQ(w, items) })
		})
	}

	entries = append(entries, encNRMeasuredResults(res)...)

	if len(entries) < 1 || len(entries) > maxNoMeas {
		return fmt.Errorf("nrppa: measured results length %d outside [1, %d]", len(entries), maxNoMeas)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxNoMeas, int64(len(entries))); err != nil {
		return err
	}

	for _, enc := range entries {
		if err := enc(w); err != nil {
			return err
		}
	}

	return nil
}

// encNRMeasuredResults returns one entry per NR quantity present, each a
// choice-Extension naming the quantity's ProtocolIE-ID.
func encNRMeasuredResults(res *ECIDResult) []func(*per.Writer) error {
	var entries []func(*per.Writer) error

	if len(res.SSRSRP) > 0 {
		items := res.SSRSRP

		entries = append(entries, encMeasuredChoiceExtension(idResultSSRSRP, func(w *per.Writer) error {
			return encRSResultList(w, len(items), func(w *per.Writer, i int) error { return encSSRSRPItem(w, items[i]) })
		}))
	}

	if len(res.SSRSRQ) > 0 {
		items := res.SSRSRQ

		entries = append(entries, encMeasuredChoiceExtension(idResultSSRSRQ, func(w *per.Writer) error {
			return encRSResultList(w, len(items), func(w *per.Writer, i int) error { return encSSRSRQItem(w, items[i]) })
		}))
	}

	if len(res.CSIRSRP) > 0 {
		items := res.CSIRSRP

		entries = append(entries, encMeasuredChoiceExtension(idResultCSIRSRP, func(w *per.Writer) error {
			return encRSResultList(w, len(items), func(w *per.Writer, i int) error { return encCSIRSRPItem(w, items[i]) })
		}))
	}

	if len(res.CSIRSRQ) > 0 {
		items := res.CSIRSRQ

		entries = append(entries, encMeasuredChoiceExtension(idResultCSIRSRQ, func(w *per.Writer) error {
			return encRSResultList(w, len(items), func(w *per.Writer, i int) error { return encCSIRSRQItem(w, items[i]) })
		}))
	}

	if res.AoA != nil {
		a := res.AoA

		entries = append(entries, encMeasuredChoiceExtension(idAngleOfArrivalNR, func(w *per.Writer) error {
			return encULAoA(w, a)
		}))
	}

	if res.NRTimingAdvance != nil {
		v := *res.NRTimingAdvance

		entries = append(entries, encMeasuredChoiceExtension(idNRTADV, func(w *per.Writer) error {
			return per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 7690, v)
		}))
	}

	if res.UERxTxTimeDiff != nil {
		v := *res.UERxTxTimeDiff

		entries = append(entries, encMeasuredChoiceExtension(idUERxTxTimeDiff, func(w *per.Writer) error {
			return per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 61565, v)
		}))
	}

	return entries
}

// encMeasuredChoiceInt covers the valueAngleOfArrival-EUTRA and
// valueTimingAdvanceType1/2-EUTRA alternatives.
func encMeasuredChoiceInt(w *per.Writer, index int, v, lb, ub int64) error {
	if err := encMeasuredChoiceIndex(w, index); err != nil {
		return err
	}

	return per.EncodeConstrainedWholeNumber(w, per.Aligned, lb, ub, v)
}

// encMeasuredChoiceList covers the resultRSRP-EUTRA and resultRSRQ-EUTRA
// alternatives.
func encMeasuredChoiceList(w *per.Writer, index int, enc func(*per.Writer) error) error {
	if err := encMeasuredChoiceIndex(w, index); err != nil {
		return err
	}

	return enc(w)
}

// encMeasuredChoiceExtension covers every NR quantity: a choice-Extension
// holding a ProtocolIE-Single-Container of id / criticality / open-type value.
func encMeasuredChoiceExtension(id ProtocolIEID, enc func(*per.Writer) error) func(*per.Writer) error {
	return func(w *per.Writer) error {
		if err := encMeasuredChoiceIndex(w, measuredChoiceExtension); err != nil {
			return err
		}

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(id)); err != nil {
			return err
		}

		if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(int(CriticalityIgnore))); err != nil {
			return err
		}

		vw := per.NewWriter()

		if err := enc(vw); err != nil {
			return fmt.Errorf("nrppa: encode measured result IE %d: %w", id, err)
		}

		return per.EncodeOpenTypeBytes(w, per.Aligned, perAlignedBytes(vw))
	}
}

func encMeasuredChoiceIndex(w *per.Writer, index int) error {
	return per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, measuredResultsAlternatives-1, int64(index))
}

// EARFCN ::= INTEGER (0..262143, ...) and NR-ARFCN ::= INTEGER (0..3279165)
// (TS 38.455 §9.2.x). Both ranges exceed 64K, so both encode with the
// indefinite length determinant of X.691 §13.2.6. TS 36.455 bounds its EARFCN
// at 65535, which fits a fixed two-octet field.
var (
	earfcnBounds  = per.Bounds{LB: 0, HasLB: true, UB: 262143, HasUB: true, Extensible: true}
	nrARFCNBounds = per.Bounds{LB: 0, HasLB: true, UB: 3279165, HasUB: true}
)

// ResultRSRP-EUTRA (TS 38.455 §9.2.30).
func encResultRSRP(w *per.Writer, items []RSRPItem) error {
	if len(items) < 1 || len(items) > maxCellReport {
		return fmt.Errorf("nrppa: RSRP items length %d outside [1, %d]", len(items), maxCellReport)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxCellReport, int64(len(items))); err != nil {
		return err
	}

	for _, it := range items {
		writeSeqPreamble(w, false, []bool{it.CGI != nil, false})

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 503, it.PCI); err != nil {
			return err
		}

		if err := per.EncodeInteger(w, per.Aligned, earfcnBounds, it.EARFCN); err != nil {
			return err
		}

		if it.CGI != nil {
			if err := encCGIEUTRA(w, *it.CGI); err != nil {
				return err
			}
		}

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 97, it.ValueRSRP); err != nil {
			return err
		}
	}

	return nil
}

// ResultRSRQ-EUTRA (TS 38.455 §9.2.31).
func encResultRSRQ(w *per.Writer, items []RSRQItem) error {
	if len(items) < 1 || len(items) > maxCellReport {
		return fmt.Errorf("nrppa: RSRQ items length %d outside [1, %d]", len(items), maxCellReport)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxCellReport, int64(len(items))); err != nil {
		return err
	}

	for _, it := range items {
		writeSeqPreamble(w, false, []bool{it.CGI != nil, false})

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 503, it.PCI); err != nil {
			return err
		}

		if err := per.EncodeInteger(w, per.Aligned, earfcnBounds, it.EARFCN); err != nil {
			return err
		}

		if it.CGI != nil {
			if err := encCGIEUTRA(w, *it.CGI); err != nil {
				return err
			}
		}

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 34, it.ValueRSRQ); err != nil {
			return err
		}
	}

	return nil
}

// encRSResultList writes the SEQUENCE (SIZE (1..maxCellReportNR)) OF wrapper the
// four NR per-cell result lists share.
func encRSResultList(w *per.Writer, n int, encItem func(*per.Writer, int) error) error {
	if n < 1 || n > maxCellReportNR {
		return fmt.Errorf("nrppa: NR result items length %d outside [1, %d]", n, maxCellReportNR)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxCellReportNR, int64(n)); err != nil {
		return err
	}

	for i := range n {
		if err := encItem(w, i); err != nil {
			return err
		}
	}

	return nil
}

// encNRCellHeader writes the nR-PCI / nR-ARFCN / cGI-NR prefix the four NR
// per-cell result items share, after their common preamble.
func encNRCellHeader(w *per.Writer, pci, arfcn int64, cgi *CGINR, hasValue, hasPer bool) error {
	writeSeqPreamble(w, false, []bool{cgi != nil, hasValue, hasPer, false})

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 1007, pci); err != nil {
		return err
	}

	// NR-ARFCN's range exceeds 64K, so it takes the indefinite-length form of
	// X.691 §13.2.6 rather than a fixed-width constrained whole number.
	if err := per.EncodeInteger(w, per.Aligned, nrARFCNBounds, arfcn); err != nil {
		return err
	}

	if cgi != nil {
		return encCGINR(w, *cgi)
	}

	return nil
}

// ResultSS-RSRP-Item (TS 38.455 §9.2.32).
func encSSRSRPItem(w *per.Writer, it SSRSRPItem) error {
	if err := encNRCellHeader(w, it.NRPCI, it.NRARFCN, it.CGI, it.Value != nil, len(it.PerSSB) > 0); err != nil {
		return err
	}

	if it.Value != nil {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 127, *it.Value); err != nil {
			return err
		}
	}

	return encPerSSB(w, it.PerSSB)
}

// ResultSS-RSRQ-Item (TS 38.455 §9.2.33).
func encSSRSRQItem(w *per.Writer, it SSRSRQItem) error {
	if err := encNRCellHeader(w, it.NRPCI, it.NRARFCN, it.CGI, it.Value != nil, len(it.PerSSB) > 0); err != nil {
		return err
	}

	if it.Value != nil {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 127, *it.Value); err != nil {
			return err
		}
	}

	return encPerSSB(w, it.PerSSB)
}

// ResultCSI-RSRP-Item (TS 38.455 §9.2.34).
func encCSIRSRPItem(w *per.Writer, it CSIRSRPItem) error {
	if err := encNRCellHeader(w, it.NRPCI, it.NRARFCN, it.CGI, it.Value != nil, len(it.PerCSIRS) > 0); err != nil {
		return err
	}

	if it.Value != nil {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 127, *it.Value); err != nil {
			return err
		}
	}

	return encPerCSIRS(w, it.PerCSIRS)
}

// ResultCSI-RSRQ-Item (TS 38.455 §9.2.35).
func encCSIRSRQItem(w *per.Writer, it CSIRSRQItem) error {
	if err := encNRCellHeader(w, it.NRPCI, it.NRARFCN, it.CGI, it.Value != nil, len(it.PerCSIRS) > 0); err != nil {
		return err
	}

	if it.Value != nil {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 127, *it.Value); err != nil {
			return err
		}
	}

	return encPerCSIRS(w, it.PerCSIRS)
}

func encPerSSB(w *per.Writer, items []SSBResultItem) error {
	if len(items) == 0 {
		return nil
	}

	if len(items) > maxIndexesReport {
		return fmt.Errorf("nrppa: per-SSB items length %d exceeds %d", len(items), maxIndexesReport)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxIndexesReport, int64(len(items))); err != nil {
		return err
	}

	for _, it := range items {
		writeSeqPreamble(w, false, []bool{false})

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 63, it.SSBIndex); err != nil {
			return err
		}

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 127, it.Value); err != nil {
			return err
		}
	}

	return nil
}

func encPerCSIRS(w *per.Writer, items []CSIRSResultItem) error {
	if len(items) == 0 {
		return nil
	}

	if len(items) > maxIndexesReport {
		return fmt.Errorf("nrppa: per-CSI-RS items length %d exceeds %d", len(items), maxIndexesReport)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxIndexesReport, int64(len(items))); err != nil {
		return err
	}

	for _, it := range items {
		writeSeqPreamble(w, false, []bool{false})

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 95, it.CSIRSIndex); err != nil {
			return err
		}

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 127, it.Value); err != nil {
			return err
		}
	}

	return nil
}

// UL-AoA (TS 38.455 §9.2.38).
func encULAoA(w *per.Writer, a *AoAResult) error {
	writeSeqPreamble(w, false, []bool{a.ZenithRaw != nil, a.LCSToGCS != nil, false})

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 3599, a.AzimuthRaw); err != nil {
		return err
	}

	if a.ZenithRaw != nil {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 1799, *a.ZenithRaw); err != nil {
			return err
		}
	}

	if a.LCSToGCS != nil {
		return encLCSToGCS(w, a.LCSToGCS)
	}

	return nil
}

// LCS-to-GCS-Translation (TS 38.455 §9.2.70). The angles travel in 0.1-degree
// units.
func encLCSToGCS(w *per.Writer, t *LCSToGCS) error {
	writeSeqPreamble(w, false, []bool{false})

	for _, deg := range []float64{t.AlphaDegrees, t.BetaDegrees, t.GammaDegrees} {
		raw := int64(deg*10 + 0.5)
		if raw < 0 || raw > 3599 {
			return fmt.Errorf("nrppa: LCS-to-GCS angle %g outside [0, 359.9] degrees", deg)
		}

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 3599, raw); err != nil {
			return err
		}
	}

	return nil
}

// Cause ::= CHOICE { radioNetwork, protocol, misc, choice-Extension }
// (TS 38.455 §9.2.1). Only the three root groups can be emitted.
func encCause(c Cause) func(*per.Writer) error {
	return func(w *per.Writer) error {
		if c.Group < CauseGroupRadioNetwork || c.Group > CauseGroupMisc {
			return fmt.Errorf("nrppa: cause group %d cannot be encoded", c.Group)
		}

		nRoot := causeGroupNRoot(c.Group)
		if c.Value < 0 || c.Value >= int64(nRoot) {
			return fmt.Errorf("nrppa: cause value %d outside group %d root [0, %d)", c.Value, c.Group, nRoot)
		}

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, causeAlternatives-1, int64(c.Group)); err != nil {
			return err
		}

		return per.EncodeEnumerated(w, per.Aligned, int64(nRoot), true, c.Value)
	}
}

// causeAlternatives counts the Cause CHOICE alternatives, choice-Extension
// included (TS 38.455 §9.2.1).
const causeAlternatives = 4

// causeGroupNRoot is the root ENUMERATED value count of each Cause group
// (TS 38.455 §9.2.1). The counts match TS 36.455 §9.2.2 group for group.
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

	for i := range nbits {
		if v&(1<<uint(nbits-1-i)) != 0 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}

	return out
}
