// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// NGAP-PDU CHOICE root alternatives (TS 38.413), in declaration order.
const (
	pduInitiatingMessage = iota
	pduSuccessfulOutcome
	pduUnsuccessfulOutcome

	pduRootCount = 3
)

// PDU is the top-level NGAP-PDU CHOICE: one of [InitiatingMessage],
// [SuccessfulOutcome], or [UnsuccessfulOutcome].
type PDU interface {
	procedureCode() ProcedureCode
	criticality() Criticality
	value() []byte
	choiceIndex() int
}

// InitiatingMessage carries a procedure's request. Value is the open-type
// payload: the APER encoding of the procedure-specific message body.
type InitiatingMessage struct {
	ProcedureCode ProcedureCode
	Criticality   Criticality
	Value         []byte
}

// SuccessfulOutcome carries a procedure's successful response.
type SuccessfulOutcome struct {
	ProcedureCode ProcedureCode
	Criticality   Criticality
	Value         []byte
}

// UnsuccessfulOutcome carries a procedure's failure response.
type UnsuccessfulOutcome struct {
	ProcedureCode ProcedureCode
	Criticality   Criticality
	Value         []byte
}

func (m *InitiatingMessage) procedureCode() ProcedureCode   { return m.ProcedureCode }
func (m *InitiatingMessage) criticality() Criticality       { return m.Criticality }
func (m *InitiatingMessage) value() []byte                  { return m.Value }
func (m *InitiatingMessage) choiceIndex() int               { return pduInitiatingMessage }
func (m *SuccessfulOutcome) procedureCode() ProcedureCode   { return m.ProcedureCode }
func (m *SuccessfulOutcome) criticality() Criticality       { return m.Criticality }
func (m *SuccessfulOutcome) value() []byte                  { return m.Value }
func (m *SuccessfulOutcome) choiceIndex() int               { return pduSuccessfulOutcome }
func (m *UnsuccessfulOutcome) procedureCode() ProcedureCode { return m.ProcedureCode }
func (m *UnsuccessfulOutcome) criticality() Criticality     { return m.Criticality }
func (m *UnsuccessfulOutcome) value() []byte                { return m.Value }
func (m *UnsuccessfulOutcome) choiceIndex() int             { return pduUnsuccessfulOutcome }

// Marshal encodes an NGAP-PDU envelope. Each of the three message alternatives
// is a non-extensible SEQUENCE { procedureCode, criticality, value } with no
// optional fields, so it carries no preamble; value is wrapped as an open type.
func Marshal(pdu PDU) ([]byte, error) {
	if pdu == nil {
		return nil, fmt.Errorf("ngap: nil PDU")
	}

	w := per.NewWriter()
	enc := per.Aligned

	w.WriteBit(false)

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, pduRootCount-1, int64(pdu.choiceIndex())); err != nil {
		return nil, err
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, 255, int64(pdu.procedureCode())); err != nil {
		return nil, err
	}

	if err := per.EncodeEnumerated(w, enc, criticalityRootCount, false, int64(pdu.criticality())); err != nil {
		return nil, err
	}

	if err := per.EncodeOpenTypeBytes(w, enc, pdu.value()); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return w.Bytes(), nil
}

// Unmarshal decodes an NGAP-PDU envelope, returning the concrete message type
// with its open-type payload in Value (decoded by the message layer).
func Unmarshal(b []byte) (PDU, error) {
	r := per.NewReader(b)

	isExt, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("ngap: PDU choice: %w", err)
	}

	if isExt {
		return nil, fmt.Errorf("ngap: unsupported NGAP-PDU extension alternative")
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, pduRootCount-1)
	if err != nil {
		return nil, fmt.Errorf("ngap: PDU choice: %w", err)
	}

	pc, crit, val, err := decodeMessageFields(r)
	if err != nil {
		return nil, err
	}

	switch idx {
	case pduInitiatingMessage:
		return &InitiatingMessage{ProcedureCode: pc, Criticality: crit, Value: val}, nil
	case pduSuccessfulOutcome:
		return &SuccessfulOutcome{ProcedureCode: pc, Criticality: crit, Value: val}, nil
	case pduUnsuccessfulOutcome:
		return &UnsuccessfulOutcome{ProcedureCode: pc, Criticality: crit, Value: val}, nil
	default:
		return nil, fmt.Errorf("ngap: unreachable PDU index %d", idx)
	}
}

func decodeMessageFields(r *per.Reader) (ProcedureCode, Criticality, []byte, error) {
	enc := per.Aligned

	pc, err := per.DecodeConstrainedWholeNumber(r, enc, 0, 255)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("ngap: procedureCode: %w", err)
	}

	crit, err := per.DecodeEnumerated(r, enc, criticalityRootCount, false)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("ngap: criticality: %w", err)
	}

	val, err := per.DecodeOpenTypeBytes(r, enc)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("ngap: value: %w", err)
	}

	return ProcedureCode(pc), Criticality(crit), val, nil
}
