// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// LPPA-PDU CHOICE root alternatives (TS 36.455 §9.3.2), in declaration order.
const (
	pduInitiatingMessage = iota
	pduSuccessfulOutcome
	pduUnsuccessfulOutcome

	pduRootCount = 3
)

// ProcedureCode ::= INTEGER (0..255). LPPa elementary procedure codes
// (TS 36.455 §9.4.3).
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

// ProtocolIEID ::= INTEGER (0..65535). LPPa ProtocolIE-ID values
// (TS 36.455 §9.4.7).
type ProtocolIEID uint16

const (
	idCause                     ProtocolIEID = 0
	idCriticalityDiagnostics    ProtocolIEID = 1
	idESMLCUEMeasurementID      ProtocolIEID = 2
	idReportCharacteristics     ProtocolIEID = 3
	idMeasurementPeriodicity    ProtocolIEID = 4
	idMeasurementQuantities     ProtocolIEID = 5
	idENBUEMeasurementID        ProtocolIEID = 6
	idECIDMeasurementResult     ProtocolIEID = 7
	idMeasurementQuantitiesItem ProtocolIEID = 11
	idCellPortionID             ProtocolIEID = 14
)

// Container/list size bounds (TS 36.455 §9.4.6).
const (
	maxProtocolIEs = 65535
	maxNoMeas      = 63
	maxCellReport  = 9
)

// lppaTransactionIDMax is the LPPATransactionID upper bound, INTEGER (0..32767)
// (TS 36.455 §9.2.3).
const lppaTransactionIDMax = 32767

// message is a decoded LPPA-PDU envelope.
type message struct {
	choiceIndex   int
	procedureCode ProcedureCode
	criticality   Criticality
	transactionID int64
	value         []byte
}

// marshalPDU encodes an LPPA-PDU envelope. Each of the three alternatives is a
// non-extensible SEQUENCE { procedureCode, criticality, lppatransactionID,
// value } with no optional fields, so it carries no preamble; value is wrapped
// as an open type (TS 36.455 §9.3.2). Every E-CID Measurement Initiation and
// Termination procedure has criticality reject (TS 36.455 §9.4.2). The E-SMLC
// uses transaction id 0; request/response correlation is by Measurement-ID.
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

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, lppaTransactionIDMax, 0); err != nil {
		return nil, err
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, body); err != nil {
		return nil, err
	}

	return perAlignedBytes(w), nil
}

// unmarshalPDU decodes an LPPA-PDU envelope, returning the concrete alternative
// with its open-type payload for the message layer to decode.
func unmarshalPDU(b []byte) (*message, error) {
	r := per.NewReader(b)

	isExt, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("lppa: PDU choice: %w", err)
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, pduRootCount-1)
	if err != nil {
		return nil, fmt.Errorf("lppa: PDU choice: %w", err)
	}

	if isExt {
		return nil, fmt.Errorf("lppa: unsupported LPPA-PDU extension alternative")
	}

	pc, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 255)
	if err != nil {
		return nil, fmt.Errorf("lppa: procedureCode: %w", err)
	}

	crit, err := decodeEnumInt(r, criticalityRootCount, false)
	if err != nil {
		return nil, fmt.Errorf("lppa: criticality: %w", err)
	}

	txn, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, lppaTransactionIDMax)
	if err != nil {
		return nil, fmt.Errorf("lppa: lppatransactionID: %w", err)
	}

	val, err := per.DecodeOpenTypeBytes(r, per.Aligned)
	if err != nil {
		return nil, fmt.Errorf("lppa: value: %w", err)
	}

	return &message{
		choiceIndex:   int(idx),
		procedureCode: ProcedureCode(pc),
		criticality:   Criticality(crit),
		transactionID: txn,
		value:         val,
	}, nil
}

// ieField is one ProtocolIE-Field to encode: its id, criticality, and a function
// that writes the value body. The engine wraps the body as an open type.
type ieField struct {
	id   ProtocolIEID
	crit Criticality
	enc  func(*per.Writer) error
}

// rawIE is a decoded ProtocolIE-Field: id, criticality, and the raw open-type
// value bytes.
type rawIE struct {
	id    ProtocolIEID
	crit  Criticality
	value []byte
}

// encodeIEContainer writes a ProtocolIE-Container (TS 36.455 §9.3.4): the field
// count as a constrained length, then each ProtocolIE-Field as
// { id, criticality, value-as-open-type } in order.
func encodeIEContainer(w *per.Writer, fields []ieField) error {
	if len(fields) > maxProtocolIEs {
		return fmt.Errorf("lppa: %d IEs exceed maxProtocolIEs", len(fields))
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
				return fmt.Errorf("lppa: encode IE %d: %w", f.id, err)
			}
		}

		if err := per.EncodeOpenTypeBytes(w, per.Aligned, perAlignedBytes(vw)); err != nil {
			return err
		}
	}

	return nil
}

// decodeIEContainer reads a ProtocolIE-Container into its fields in wire order,
// preserving every field for dispatch by id.
func decodeIEContainer(r *per.Reader) ([]rawIE, error) {
	n, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs)
	if err != nil {
		return nil, fmt.Errorf("lppa: IE container length: %w", err)
	}

	var fields []rawIE

	for i := int64(0); i < n; i++ {
		id, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, maxProtocolIEs)
		if err != nil {
			return nil, fmt.Errorf("lppa: IE %d id: %w", i, err)
		}

		crit, err := decodeEnumInt(r, criticalityRootCount, false)
		if err != nil {
			return nil, fmt.Errorf("lppa: IE %d criticality: %w", i, err)
		}

		val, err := per.DecodeOpenTypeBytes(r, per.Aligned)
		if err != nil {
			return nil, fmt.Errorf("lppa: IE %d value: %w", i, err)
		}

		fields = append(fields, rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: val})
	}

	return fields, nil
}

// writeSeqPreamble writes a SEQUENCE preamble: the extension bit followed by
// one presence bit per OPTIONAL root field.
//
//nolint:unparam
func writeSeqPreamble(w *per.Writer, extPresent bool, optionals []bool) {
	w.WriteBit(extPresent)

	for _, present := range optionals {
		w.WriteBit(present)
	}
}

// readSeqPreamble reads a SEQUENCE preamble with nOptional presence bits.
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

// skipExtensionAdditions consumes an extension-addition block: the
// normally-small-length bitmap followed by that many open-type fields.
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

// decodeChoiceIndex decodes an extensible CHOICE index (X.691 §23): the
// extension marker, then either a root index as a constrained whole number or,
// for an extension alternative, a normally-small number. Every CHOICE in LPPa
// is extensible.
func decodeChoiceIndex(r *per.Reader, nRoot int64) (index int64, isExt bool, err error) {
	bit, err := r.ReadBit()
	if err != nil {
		return 0, false, err
	}

	if bit {
		v, err := per.DecodeNormallySmall(r, per.Aligned)

		return v, true, err
	}

	index, err = per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, nRoot-1)

	return index, false, err
}

// decodeEnumInt decodes an ENUMERATED value as an int index.
func decodeEnumInt(r *per.Reader, nRoot int, extensible bool) (int, error) {
	v, err := per.DecodeEnumerated(r, per.Aligned, int64(nRoot), extensible)

	return int(v), err
}

// writeExtConstrainedInt encodes an extensible constrained INTEGER's root value:
// the extension marker (0) followed by the constrained value (X.691 §12.2.6). It
// never emits an extension value.
func writeExtConstrainedInt(w *per.Writer, v, lb, ub int64) error {
	w.WriteBit(false)
	return per.EncodeConstrainedWholeNumber(w, per.Aligned, lb, ub, v)
}

// readExtConstrainedInt decodes an extensible constrained INTEGER. A root value
// (marker 0) reads over [lb, ub]; an extension value (marker 1) reads as an
// unconstrained integer (X.691 §12.2.6).
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
