// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"iter"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

var (
	ErrIENotPresent = ap.ErrIENotPresent
	ErrDuplicateIE  = ap.ErrDuplicateIE
)

type RawPDU struct {
	m *ap.RawPDU[ProtocolIEID]
}

func ParseRawPDU(b []byte) (*RawPDU, error) {
	m, err := ap.ParseRawPDU[ProtocolIEID](profile, b)
	if err != nil {
		return nil, err
	}

	return &RawPDU{m: m}, nil
}

func (p *RawPDU) Trigger() TriggeringMessage { return TriggeringMessage(p.m.Choice()) }

func (p *RawPDU) ProcedureCode() ProcedureCode { return ProcedureCode(p.m.ProcedureCode()) }

func (p *RawPDU) Criticality() Criticality { return p.m.Criticality() }

func (p *RawPDU) Opaque() bool { return p.m.Opaque() }

func (p *RawPDU) Body() []byte { return p.m.Body() }

func (p *RawPDU) Len() int { return p.m.Len() }

func (p *RawPDU) IE(i int) (RawIE, error) { return p.m.Field(i) }

func (p *RawPDU) IEs() []RawIE { return p.m.Fields() }

func (p *RawPDU) All(id ProtocolIEID) iter.Seq2[int, RawIE] { return p.m.All(id) }

func (p *RawPDU) Index(id ProtocolIEID) (int, error) { return p.m.Index(id) }

func (p *RawPDU) SetValue(i int, value []byte) error { return p.m.SetValue(i, value) }

func (p *RawPDU) Insert(i int, ie RawIE) error { return p.m.Insert(i, ie) }

func (p *RawPDU) Remove(i int) error { return p.m.Remove(i) }

func (p *RawPDU) Marshal() ([]byte, error) { return p.m.Marshal() }

func (p *RawPDU) Get(id ProtocolIEID, v per.Unmarshaler) (bool, error) { return p.m.Get(id, v) }

func (p *RawPDU) Set(id ProtocolIEID, v per.Marshaler) error { return p.m.Set(id, v) }
