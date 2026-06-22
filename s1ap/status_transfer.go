// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// StatusTransferContainer holds the eNB Status Transfer Transparent Container
// (TS 36.413 §9.2.1.x) as its raw open-type value bytes. The MME does not
// interpret the PDCP-SN/HFN COUNT values it carries; it relays the container
// verbatim from ENB STATUS TRANSFER into MME STATUS TRANSFER (§8.4.6/§8.4.7).
type StatusTransferContainer []byte

func (c StatusTransferContainer) field(id ProtocolIEID) ieField {
	return ieField{id: id, crit: CriticalityReject, enc: func(w *aper.Writer) error {
		w.WriteOctets(c)
		return nil
	}}
}

// ENBStatusTransfer is the ENB STATUS TRANSFER message (TS 36.413 §9.1.5... in
// the eNB Status Transfer procedure), sent by the source eNB to convey PDCP-SN
// and HFN status to the target eNB via the MME.
type ENBStatusTransfer struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Container   StatusTransferContainer

	unmodeledIEs
}

func (m *ENBStatusTransfer) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		m.Container.field(idENBStatusTransferTransparentContainer),
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ENBStatusTransfer) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcENBStatusTransfer,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseENBStatusTransfer decodes the message from an initiatingMessage open-type
// payload.
func ParseENBStatusTransfer(value []byte) (*ENBStatusTransfer, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBStatusTransfer preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &ENBStatusTransfer{}

	var seenMME, seenENB, seenContainer bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idENBStatusTransferTransparentContainer:
			m.Container = StatusTransferContainer(f.value)
			seenContainer = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: ENBStatusTransfer IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenContainer {
		return nil, fmt.Errorf("s1ap: ENBStatusTransfer missing mandatory IE")
	}

	return m, nil
}

// MMEStatusTransfer is the MME STATUS TRANSFER message (TS 36.413 §9.1.5... in
// the MME Status Transfer procedure), sent by the MME to relay the source eNB's
// status container to the target eNB.
type MMEStatusTransfer struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Container   StatusTransferContainer

	unmodeledIEs
}

func (m *MMEStatusTransfer) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		m.Container.field(idENBStatusTransferTransparentContainer),
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *MMEStatusTransfer) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcMMEStatusTransfer,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseMMEStatusTransfer decodes the message from an initiatingMessage open-type
// payload.
func ParseMMEStatusTransfer(value []byte) (*MMEStatusTransfer, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: MMEStatusTransfer preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &MMEStatusTransfer{}

	var seenMME, seenENB, seenContainer bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idENBStatusTransferTransparentContainer:
			m.Container = StatusTransferContainer(f.value)
			seenContainer = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: MMEStatusTransfer IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenContainer {
		return nil, fmt.Errorf("s1ap: MMEStatusTransfer missing mandatory IE")
	}

	return m, nil
}
