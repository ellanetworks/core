// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// StatusTransferContainer holds the eNB Status Transfer Transparent Container
// (TS 36.413) as its raw open-type value bytes. The MME does not
// interpret the PDCP-SN/HFN COUNT values it carries; it relays the container
// verbatim from ENB STATUS TRANSFER into MME STATUS TRANSFER.
type StatusTransferContainer []byte

func (c StatusTransferContainer) field(id ProtocolIEID) ieField {
	return ieField{id: id, crit: CriticalityReject, raw: c}
}

// ENBStatusTransfer is the ENB STATUS TRANSFER message (TS 36.413 in
// the eNB Status Transfer procedure), sent by the source eNB to convey PDCP-SN
// and HFN status to the target eNB via the MME.
type ENBStatusTransfer struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Container   StatusTransferContainer

	unmodeledIEs
}

func (m *ENBStatusTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		m.Container.field(idENBStatusTransferTransparentContainer),
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *ENBStatusTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcENBStatusTransfer,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseENBStatusTransfer decodes the message from an initiatingMessage open-type
// payload.
func ParseENBStatusTransfer(value []byte) (*ENBStatusTransfer, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBStatusTransfer preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &ENBStatusTransfer{}

	var seenMME, seenENB, seenContainer bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
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

	if err := requireIEs(ProcENBStatusTransfer,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idENBStatusTransferTransparentContainer, CriticalityReject, seenContainer},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// MMEStatusTransfer is the MME STATUS TRANSFER message (TS 36.413 in
// the MME Status Transfer procedure), sent by the MME to relay the source eNB's
// status container to the target eNB.
type MMEStatusTransfer struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Container   StatusTransferContainer

	unmodeledIEs
}

func (m *MMEStatusTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		m.Container.field(idENBStatusTransferTransparentContainer),
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *MMEStatusTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcMMEStatusTransfer,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseMMEStatusTransfer decodes the message from an initiatingMessage open-type
// payload.
func ParseMMEStatusTransfer(value []byte) (*MMEStatusTransfer, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: MMEStatusTransfer preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &MMEStatusTransfer{}

	var seenMME, seenENB, seenContainer bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
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

	if err := requireIEs(ProcMMEStatusTransfer,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idENBStatusTransferTransparentContainer, CriticalityReject, seenContainer},
	); err != nil {
		return nil, err
	}

	return m, nil
}
