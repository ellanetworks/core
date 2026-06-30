// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// S1AP-PDU CHOICE root alternatives (TS 36.413), in declaration order.
const (
	pduInitiatingMessage = iota
	pduSuccessfulOutcome
	pduUnsuccessfulOutcome

	pduRootCount = 3
)

// PDU is the top-level S1AP-PDU CHOICE: one of [InitiatingMessage],
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

// Marshal encodes an S1AP-PDU envelope. Each of the three message alternatives
// is a non-extensible SEQUENCE { procedureCode, criticality, value } with no
// optional fields, so it carries no preamble; value is wrapped as an open type.
func Marshal(pdu PDU) ([]byte, error) {
	if pdu == nil {
		return nil, fmt.Errorf("s1ap: nil PDU")
	}

	var w aper.Writer

	if err := w.WriteChoiceIndex(pdu.choiceIndex(), pduRootCount, true, false); err != nil {
		return nil, err
	}

	if err := w.WriteConstrainedInt(int64(pdu.procedureCode()), 0, 255); err != nil {
		return nil, err
	}

	if err := w.WriteEnum(int(pdu.criticality()), criticalityRootCount, false, false); err != nil {
		return nil, err
	}

	if err := w.WriteOpenType(pdu.value()); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// Unmarshal decodes an S1AP-PDU envelope, returning the concrete message type
// with its open-type payload in Value (decoded by the message layer).
func Unmarshal(b []byte) (PDU, error) {
	r := aper.NewReader(b)

	idx, isExt, err := r.ReadChoiceIndex(pduRootCount, true)
	if err != nil {
		return nil, fmt.Errorf("s1ap: PDU choice: %w", err)
	}

	if isExt {
		return nil, fmt.Errorf("s1ap: unsupported S1AP-PDU extension alternative")
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
		return nil, fmt.Errorf("s1ap: unreachable PDU index %d", idx)
	}
}

func decodeMessageFields(r *aper.Reader) (ProcedureCode, Criticality, []byte, error) {
	pc, err := r.ReadConstrainedInt(0, 255)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("s1ap: procedureCode: %w", err)
	}

	crit, _, err := r.ReadEnum(criticalityRootCount, false)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("s1ap: criticality: %w", err)
	}

	val, err := r.ReadOpenType()
	if err != nil {
		return 0, 0, nil, fmt.Errorf("s1ap: value: %w", err)
	}

	return ProcedureCode(pc), Criticality(crit), val, nil
}
