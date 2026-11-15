// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/per/ap/apimpl"
)

// NGAP-PDU CHOICE root alternatives (TS 38.413), in declaration order.
const (
	pduInitiatingMessage = iota
	pduSuccessfulOutcome
	pduUnsuccessfulOutcome
)

// PDU is the top-level NGAP-PDU CHOICE: one of [InitiatingMessage],
// [SuccessfulOutcome], or [UnsuccessfulOutcome].
type PDU interface {
	procedureCode() ProcedureCode
	criticality() Criticality
	value() []byte
	choiceIndex() int
}

// Value is the open-type payload: the APER encoding of the message body.
type InitiatingMessage struct {
	ProcedureCode ProcedureCode
	Criticality   Criticality
	Value         []byte
}

type SuccessfulOutcome struct {
	ProcedureCode ProcedureCode
	Criticality   Criticality
	Value         []byte
}

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

func (m InitiatingMessage) Equal(o InitiatingMessage) bool {
	return m.ProcedureCode == o.ProcedureCode && m.Criticality == o.Criticality && bytes.Equal(m.Value, o.Value)
}

func (m SuccessfulOutcome) Equal(o SuccessfulOutcome) bool {
	return m.ProcedureCode == o.ProcedureCode && m.Criticality == o.Criticality && bytes.Equal(m.Value, o.Value)
}

func (m UnsuccessfulOutcome) Equal(o UnsuccessfulOutcome) bool {
	return m.ProcedureCode == o.ProcedureCode && m.Criticality == o.Criticality && bytes.Equal(m.Value, o.Value)
}

// Each alternative is a non-extensible SEQUENCE with no optional fields, so it
// carries no preamble; value is wrapped as an open type.
func Marshal(pdu PDU) ([]byte, error) {
	if pdu == nil {
		return nil, fmt.Errorf("ngap: nil PDU")
	}

	return apimpl.EncodePDU(profile, apimpl.Envelope{
		Choice:        pdu.choiceIndex(),
		ProcedureCode: uint8(pdu.procedureCode()),
		Criticality:   pdu.criticality(),
		Value:         pdu.value(),
	})
}

// The open-type payload is left in Value for the message layer.
func Unmarshal(b []byte) (PDU, error) {
	e, err := apimpl.DecodePDU[ProcedureCode](profile, b)
	if err != nil {
		return nil, err
	}

	pc := ProcedureCode(e.ProcedureCode)

	switch e.Choice {
	case pduInitiatingMessage:
		return &InitiatingMessage{ProcedureCode: pc, Criticality: e.Criticality, Value: e.Value}, nil
	case pduSuccessfulOutcome:
		return &SuccessfulOutcome{ProcedureCode: pc, Criticality: e.Criticality, Value: e.Value}, nil
	default:
		return &UnsuccessfulOutcome{ProcedureCode: pc, Criticality: e.Criticality, Value: e.Value}, nil
	}
}
