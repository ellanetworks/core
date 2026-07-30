// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
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

func (m *HandoverNotify) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idEUTRANCGI, crit: CriticalityIgnore, val: &m.EUTRANCGI},
		{id: idTAI, crit: CriticalityIgnore, val: &m.TAI},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *HandoverNotify) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverNotification,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseHandoverNotify decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverNotify(value []byte) (*HandoverNotify, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverNotify preamble: %w", err)
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

	m := &HandoverNotify{}

	var seenMME, seenENB, seenCGI, seenTAI bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idEUTRANCGI:
			err = perIEDecode(f.value, &m.EUTRANCGI)
			seenCGI = true
		case idTAI:
			err = perIEDecode(f.value, &m.TAI)
			seenTAI = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverNotify IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcHandoverNotification,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idEUTRANCGI, CriticalityIgnore, seenCGI},
		ieCheck{idTAI, CriticalityIgnore, seenTAI},
	); err != nil {
		return nil, err
	}

	return m, nil
}
