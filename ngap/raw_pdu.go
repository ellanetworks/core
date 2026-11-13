// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/protocolie"
)

var ErrIENotPresent = errors.New("ngap: IE not present")

type RawPDU struct {
	m *protocolie.Message
}

func ParseRawPDU(b []byte) (*RawPDU, error) {
	m, err := protocolie.Parse(b, func(pc uint8) bool { return ProcedureCode(pc) == ProcPrivateMessage })
	if err != nil {
		return nil, fmt.Errorf("ngap: %w", err)
	}

	return &RawPDU{m: m}, nil
}

func (p *RawPDU) Trigger() TriggeringMessage { return TriggeringMessage(p.m.Choice()) }

func (p *RawPDU) ProcedureCode() ProcedureCode { return ProcedureCode(p.m.ProcedureCode()) }

func (p *RawPDU) Criticality() Criticality { return Criticality(p.m.Criticality()) }

func (p *RawPDU) Len() int { return p.m.Len() }

func (p *RawPDU) IE(i int) (RawIE, error) {
	f, err := p.m.Field(i)
	if err != nil {
		return RawIE{}, fmt.Errorf("ngap: %w", err)
	}

	return RawIE{ID: ProtocolIEID(f.ID), Criticality: Criticality(f.Criticality), Value: f.Value}, nil
}

func (p *RawPDU) IEs() []RawIE {
	fields := p.m.Fields()

	out := make([]RawIE, len(fields))
	for i, f := range fields {
		out[i] = RawIE{ID: ProtocolIEID(f.ID), Criticality: Criticality(f.Criticality), Value: f.Value}
	}

	return out
}

func (p *RawPDU) Index(id ProtocolIEID) int { return p.m.Index(uint16(id)) }

func (p *RawPDU) SetValue(i int, value []byte) error {
	if err := p.m.SetValue(i, value); err != nil {
		return fmt.Errorf("ngap: %w", err)
	}

	return nil
}

func (p *RawPDU) Insert(i int, ie RawIE) error {
	if err := p.m.Insert(i, protocolie.Field{ID: uint16(ie.ID), Criticality: uint8(ie.Criticality), Value: ie.Value}); err != nil {
		return fmt.Errorf("ngap: %w", err)
	}

	return nil
}

func (p *RawPDU) Remove(i int) error {
	if err := p.m.Remove(i); err != nil {
		return fmt.Errorf("ngap: %w", err)
	}

	return nil
}

func (p *RawPDU) Marshal() ([]byte, error) {
	b, err := p.m.Bytes()
	if err != nil {
		return nil, fmt.Errorf("ngap: %w", err)
	}

	return b, nil
}

func (p *RawPDU) Get(id ProtocolIEID, v per.Unmarshaler) (bool, error) {
	i := p.m.Index(uint16(id))
	if i < 0 {
		return false, nil
	}

	f, err := p.m.Field(i)
	if err != nil {
		return false, fmt.Errorf("ngap: %w", err)
	}

	if err := perIEDecode(f.Value, v); err != nil {
		return false, fmt.Errorf("ngap: IE %s: %w", id, err)
	}

	return true, nil
}

func (p *RawPDU) Set(id ProtocolIEID, v per.Marshaler) error {
	i := p.m.Index(uint16(id))
	if i < 0 {
		return fmt.Errorf("%w: %s", ErrIENotPresent, id)
	}

	b, err := per.Marshal(v, per.Aligned)
	if err != nil {
		return fmt.Errorf("ngap: IE %s: %w", id, err)
	}

	return p.SetValue(i, b)
}
