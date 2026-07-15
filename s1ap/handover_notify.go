// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

// HandoverNotify is the HANDOVER NOTIFY message (TS 36.413 in the
// Handover Notification procedure), sent by the target eNB once the UE has
// arrived in the target cell and the S1 handover is complete (TS 23.401).
// It carries the target eNB's UE S1AP ID and the UE's new
// location.
type HandoverNotify struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	EUTRANCGI   EUTRANCGI
	TAI         TAI

	unmodeledIEs
}

func (m *HandoverNotify) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idEUTRANCGI, crit: CriticalityIgnore, enc: m.EUTRANCGI.encode},
		{id: idTAI, crit: CriticalityIgnore, enc: m.TAI.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverNotify) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverNotification,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseHandoverNotify decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverNotify(value []byte) (*HandoverNotify, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverNotify preamble: %w", err)
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

	m := &HandoverNotify{}

	var seenMME, seenENB, seenCGI, seenTAI bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idEUTRANCGI:
			m.EUTRANCGI, err = decodeEUTRANCGI(sub)
			seenCGI = true
		case idTAI:
			m.TAI, err = decodeTAI(sub)
			seenTAI = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverNotify IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenCGI || !seenTAI {
		return nil, fmt.Errorf("s1ap: HandoverNotify missing mandatory IE")
	}

	return m, nil
}
