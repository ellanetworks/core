// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// NRPPA-PDU CHOICE root alternatives, in ASN.1 order (TS 38.455 §9.3.2).
const (
	pduInitiatingMessage = iota
	pduSuccessfulOutcome
	pduUnsuccessfulOutcome

	pduRootCount = 3
)

// ProcedureCode ::= INTEGER (0..255) (TS 38.455 §9.4.3). The E-CID codes match
// TS 36.455 §9.4.3 one for one.
type ProcedureCode uint8

const (
	ProcErrorIndication                  ProcedureCode = 0
	ProcPrivateMessage                   ProcedureCode = 1
	ProcECIDMeasurementInitiation        ProcedureCode = 2
	ProcECIDMeasurementFailureIndication ProcedureCode = 3
	ProcECIDMeasurementReport            ProcedureCode = 4
	ProcECIDMeasurementTermination       ProcedureCode = 5
)

// Criticality ::= ENUMERATED { reject, ignore, notify } (not extensible).
type Criticality uint8

const (
	CriticalityReject Criticality = iota
	CriticalityIgnore
	CriticalityNotify

	criticalityRootCount = 3
)

// ProtocolIEID ::= INTEGER (0..65535) (TS 38.455 §9.4.7). The ids this codec
// cites are the E-CID ones, which match TS 36.455 §9.4.7 apart from the NR
// measurement results, which LPPa does not define.
type ProtocolIEID uint16

const (
	idCause                     ProtocolIEID = 0
	idCriticalityDiagnostics    ProtocolIEID = 1
	idLMFUEMeasurementID        ProtocolIEID = 2
	idReportCharacteristics     ProtocolIEID = 3
	idMeasurementPeriodicity    ProtocolIEID = 4
	idMeasurementQuantities     ProtocolIEID = 5
	idRANUEMeasurementID        ProtocolIEID = 6
	idECIDMeasurementResult     ProtocolIEID = 7
	idMeasurementQuantitiesItem ProtocolIEID = 11
	idCellPortionID             ProtocolIEID = 14
	idResultSSRSRP              ProtocolIEID = 32
	idResultSSRSRQ              ProtocolIEID = 33
	idResultCSIRSRP             ProtocolIEID = 34
	idResultCSIRSRQ             ProtocolIEID = 35
	idAngleOfArrivalNR          ProtocolIEID = 36
	idNRTADV                    ProtocolIEID = 94
	idUERxTxTimeDiff            ProtocolIEID = 118
)

// Container/list size bounds (TS 38.455 §9.4.6).
const (
	maxProtocolIEs   = 65535
	maxNoMeas        = 64 // TS 36.455 has 63
	maxCellReport    = 9
	maxCellReportNR  = 9
	maxIndexesReport = 64
)

// NRPPATransactionID ::= INTEGER (0..32767) (TS 38.455 §9.2.4).
const nrppaTransactionIDMax = 32767

// message is a decoded NRPPA-PDU envelope.
type message struct {
	choiceIndex   int
	procedureCode ProcedureCode
	criticality   Criticality
	transactionID int64
	value         []byte
}

// marshalPDU encodes an NRPPA-PDU envelope (TS 38.455 §9.3.2). All three
// alternatives are non-extensible SEQUENCEs with no optional field, so none
// carries a preamble. Every procedure this package emits has criticality reject
// (TS 38.455 §9.4.2), and the LMF always uses transaction id 0.
func marshalPDU(choiceIndex int, pc ProcedureCode, body []byte) ([]byte, error) {
	w := per.NewWriter()

	if err := func() error {
		w.WriteBit(false)
		return per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, pduRootCount-1, int64(choiceIndex))
	}(); err != nil {
		return nil, err
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 255, int64(pc)); err != nil {
		return nil, err
	}

	if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(int(CriticalityReject))); err != nil {
		return nil, err
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, nrppaTransactionIDMax, 0); err != nil {
		return nil, err
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, body); err != nil {
		return nil, err
	}

	return perAlignedBytes(w), nil
}

// unmarshalPDU leaves the open-type payload undecoded in message.value.
func unmarshalPDU(b []byte) (*message, error) {
	r := per.NewReader(b)

	isExt, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("nrppa: PDU choice: %w", err)
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, pduRootCount-1)
	if err != nil {
		return nil, fmt.Errorf("nrppa: PDU choice: %w", err)
	}

	if isExt {
		return nil, fmt.Errorf("nrppa: unsupported NRPPA-PDU extension alternative")
	}

	pc, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 255)
	if err != nil {
		return nil, fmt.Errorf("nrppa: procedureCode: %w", err)
	}

	crit, err := decodeEnumInt(r, criticalityRootCount, false)
	if err != nil {
		return nil, fmt.Errorf("nrppa: criticality: %w", err)
	}

	txn, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, nrppaTransactionIDMax)
	if err != nil {
		return nil, fmt.Errorf("nrppa: nrppatransactionID: %w", err)
	}

	val, err := per.DecodeOpenTypeBytes(r, per.Aligned)
	if err != nil {
		return nil, fmt.Errorf("nrppa: value: %w", err)
	}

	return &message{
		choiceIndex:   int(idx),
		procedureCode: ProcedureCode(pc),
		criticality:   Criticality(crit),
		transactionID: txn,
		value:         val,
	}, nil
}

// ieField is a ProtocolIE-Field to encode; enc writes the bare value, which
// encodeIEContainer wraps as an open type.
type ieField struct {
	id   ProtocolIEID
	crit Criticality
	enc  func(*per.Writer) error
}

// rawIE is a decoded ProtocolIE-Field with its value left as open-type bytes.
type rawIE struct {
	id    ProtocolIEID
	crit  Criticality
	value []byte
}

// encodeIEContainer writes a ProtocolIE-Container (TS 38.455 §9.3.4).
func encodeIEContainer(w *per.Writer, fields []ieField) error {
	if len(fields) > maxProtocolIEs {
		return fmt.Errorf("nrppa: %d IEs exceed maxProtocolIEs", len(fields))
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(len(fields))); err != nil {
		return err
	}

	for _, f := range fields {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(f.id)); err != nil {
			return err
		}

		if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(int(f.crit))); err != nil {
			return err
		}

		vw := per.NewWriter()

		if f.enc != nil {
			if err := f.enc(vw); err != nil {
				return fmt.Errorf("nrppa: encode IE %d: %w", f.id, err)
			}
		}

		if err := per.EncodeOpenTypeBytes(w, per.Aligned, perAlignedBytes(vw)); err != nil {
			return err
		}
	}

	return nil
}

// decodeIEContainer keeps every field, including ids this codec does not model.
func decodeIEContainer(r *per.Reader) ([]rawIE, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs)
	if err != nil {
		return nil, fmt.Errorf("nrppa: IE container length: %w", err)
	}

	var fields []rawIE

	for i := int64(0); i < n; i++ {
		id, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs)
		if err != nil {
			return nil, fmt.Errorf("nrppa: IE %d id: %w", i, err)
		}

		crit, err := decodeEnumInt(r, criticalityRootCount, false)
		if err != nil {
			return nil, fmt.Errorf("nrppa: IE %d criticality: %w", i, err)
		}

		val, err := per.DecodeOpenTypeBytes(r, per.Aligned)
		if err != nil {
			return nil, fmt.Errorf("nrppa: IE %d value: %w", i, err)
		}

		fields = append(fields, rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: val})
	}

	return fields, nil
}

// writeSeqPreamble writes the extension bit then one presence bit per OPTIONAL
// root field.
//
//nolint:unparam
func writeSeqPreamble(w *per.Writer, extPresent bool, optionals []bool) {
	w.WriteBit(extPresent)

	for _, present := range optionals {
		w.WriteBit(present)
	}
}

func readSeqPreamble(r *per.Reader, nOptional int) (bool, []bool, error) {
	extPresent, err := r.ReadBit()
	if err != nil {
		return false, nil, err
	}

	optionals := make([]bool, nOptional)
	for i := range optionals {
		optionals[i], err = r.ReadBit()
		if err != nil {
			return false, nil, err
		}
	}

	return extPresent, optionals, nil
}

// skipExtensionAdditions consumes the normally-small-length presence bitmap and
// the open type of each present addition.
func skipExtensionAdditions(r *per.Reader) error {
	var present []bool

	err := per.DecodeNormallySmallLength(r, per.Aligned, func(count int64) error {
		present = make([]bool, count)
		for i := range present {
			b, err := r.ReadBit()
			if err != nil {
				return err
			}

			present[i] = b
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, p := range present {
		if p {
			if err := per.SkipOpenType(r, per.Aligned); err != nil {
				return err
			}
		}
	}

	return nil
}

// perAlignedBytes pads w to an octet boundary and returns its bytes.
func perAlignedBytes(w *per.Writer) []byte {
	w.AlignToByte()

	return w.Bytes()
}

func decodeEnumInt(r *per.Reader, nRoot int, extensible bool) (int, error) {
	v, err := per.DecodeEnumerated(r, per.Aligned, int64(nRoot), extensible)

	return int(v), err
}

// writeExtConstrainedInt encodes an extensible constrained INTEGER
// (X.691 §12.2.6), always as a root value.
func writeExtConstrainedInt(w *per.Writer, v, lb, ub int64) error {
	w.WriteBit(false)
	return per.EncodeConstrainedWholeNumber(w, per.Aligned, lb, ub, v)
}

// readExtConstrainedInt reads an extension value as an unconstrained integer,
// a root value over [lb, ub] (X.691 §12.2.6).
func readExtConstrainedInt(r *per.Reader, lb, ub int64) (int64, error) {
	b, err := r.ReadBit()
	if err != nil {
		return 0, err
	}

	if b {
		return per.DecodeInteger(r, per.Aligned, per.Bounds{})
	}

	return per.DecodeConstrainedWholeNumber(r, per.Aligned, lb, ub)
}
