// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// HandoverCancel is the HANDOVER CANCEL message (TS 36.413 §9.1.5.11), sent by
// the source eNB to cancel an ongoing or prepared handover (TS 23.401
// §5.5.1.2.4).
type HandoverCancel struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       Cause

	unmodeledIEs
}

func (m *HandoverCancel) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverCancel) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverCancel,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverCancel decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverCancel(value []byte) (*HandoverCancel, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverCancel preamble: %w", err)
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

	m := &HandoverCancel{}

	var seenMME, seenENB, seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverCancel IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenCause {
		return nil, fmt.Errorf("s1ap: HandoverCancel missing mandatory IE")
	}

	return m, nil
}

// HandoverCancelAcknowledge is the HANDOVER CANCEL ACKNOWLEDGE message (TS 36.413
// §9.1.5.12), the successful outcome the MME returns to confirm the handover has
// been cancelled and target resources released.
type HandoverCancelAcknowledge struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID

	unmodeledIEs
}

func (m *HandoverCancelAcknowledge) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityIgnore, enc: m.ENBUES1APID.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverCancelAcknowledge) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverCancel,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseHandoverCancelAcknowledge decodes the message from a successfulOutcome
// open-type payload.
func ParseHandoverCancelAcknowledge(value []byte) (*HandoverCancelAcknowledge, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverCancelAcknowledge preamble: %w", err)
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

	m := &HandoverCancelAcknowledge{}

	var seenMME, seenENB bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverCancelAcknowledge IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB {
		return nil, fmt.Errorf("s1ap: HandoverCancelAcknowledge missing mandatory IE")
	}

	return m, nil
}
