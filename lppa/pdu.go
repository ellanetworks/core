// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
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
	var w aper.Writer

	if err := w.WriteChoiceIndex(choiceIndex, pduRootCount, true, false); err != nil {
		return nil, err
	}

	if err := w.WriteConstrainedInt(int64(pc), 0, 255); err != nil {
		return nil, err
	}

	if err := w.WriteEnum(int(CriticalityReject), criticalityRootCount, false, false); err != nil {
		return nil, err
	}

	if err := w.WriteConstrainedInt(0, 0, lppaTransactionIDMax); err != nil {
		return nil, err
	}

	if err := w.WriteOpenType(body); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// unmarshalPDU decodes an LPPA-PDU envelope, returning the concrete alternative
// with its open-type payload for the message layer to decode.
func unmarshalPDU(b []byte) (*message, error) {
	r := aper.NewReader(b)

	idx, isExt, err := r.ReadChoiceIndex(pduRootCount, true)
	if err != nil {
		return nil, fmt.Errorf("lppa: PDU choice: %w", err)
	}

	if isExt {
		return nil, fmt.Errorf("lppa: unsupported LPPA-PDU extension alternative")
	}

	pc, err := r.ReadConstrainedInt(0, 255)
	if err != nil {
		return nil, fmt.Errorf("lppa: procedureCode: %w", err)
	}

	crit, _, err := r.ReadEnum(criticalityRootCount, false)
	if err != nil {
		return nil, fmt.Errorf("lppa: criticality: %w", err)
	}

	txn, err := r.ReadConstrainedInt(0, lppaTransactionIDMax)
	if err != nil {
		return nil, fmt.Errorf("lppa: lppatransactionID: %w", err)
	}

	val, err := r.ReadOpenType()
	if err != nil {
		return nil, fmt.Errorf("lppa: value: %w", err)
	}

	return &message{
		choiceIndex:   idx,
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
	enc  func(*aper.Writer) error
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
func encodeIEContainer(w *aper.Writer, fields []ieField) error {
	if len(fields) > maxProtocolIEs {
		return fmt.Errorf("lppa: %d IEs exceed maxProtocolIEs", len(fields))
	}

	if err := w.WriteConstrainedLength(len(fields), 0, maxProtocolIEs); err != nil {
		return err
	}

	for _, f := range fields {
		if err := w.WriteConstrainedInt(int64(f.id), 0, maxProtocolIEs); err != nil {
			return err
		}

		if err := w.WriteEnum(int(f.crit), criticalityRootCount, false, false); err != nil {
			return err
		}

		var vw aper.Writer

		if f.enc != nil {
			if err := f.enc(&vw); err != nil {
				return fmt.Errorf("lppa: encode IE %d: %w", f.id, err)
			}
		}

		if err := w.WriteOpenType(vw.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

// decodeIEContainer reads a ProtocolIE-Container into its fields in wire order,
// preserving every field for dispatch by id.
func decodeIEContainer(r *aper.Reader) ([]rawIE, error) {
	n, err := r.ReadConstrainedLength(0, maxProtocolIEs)
	if err != nil {
		return nil, fmt.Errorf("lppa: IE container length: %w", err)
	}

	var fields []rawIE

	for i := 0; i < n; i++ {
		id, err := r.ReadConstrainedInt(0, maxProtocolIEs)
		if err != nil {
			return nil, fmt.Errorf("lppa: IE %d id: %w", i, err)
		}

		crit, _, err := r.ReadEnum(criticalityRootCount, false)
		if err != nil {
			return nil, fmt.Errorf("lppa: IE %d criticality: %w", i, err)
		}

		val, err := r.ReadOpenType()
		if err != nil {
			return nil, fmt.Errorf("lppa: IE %d value: %w", i, err)
		}

		fields = append(fields, rawIE{id: ProtocolIEID(id), crit: Criticality(crit), value: val})
	}

	return fields, nil
}

// writeExtConstrainedInt encodes an extensible constrained INTEGER's root value:
// the extension marker (0) followed by the constrained value (X.691 §12.2.6). It
// never emits an extension value.
func writeExtConstrainedInt(w *aper.Writer, v, lb, ub int64) error {
	w.WriteBit(0)
	return w.WriteConstrainedInt(v, lb, ub)
}

// readExtConstrainedInt decodes an extensible constrained INTEGER. A root value
// (marker 0) reads over [lb, ub]; an extension value (marker 1) reads as an
// unconstrained integer (X.691 §12.2.6).
func readExtConstrainedInt(r *aper.Reader, lb, ub int64) (int64, error) {
	b, err := r.ReadBit()
	if err != nil {
		return 0, err
	}

	if b == 1 {
		return r.ReadUnconstrainedInt()
	}

	return r.ReadConstrainedInt(lb, ub)
}
