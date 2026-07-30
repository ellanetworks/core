// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
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

// handoverNotifyIEs is the HandoverNotify IE table (TS 36.413).
var handoverNotifyIEs = []ieSpec[HandoverNotify]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idEUTRANCGI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.EUTRANCGI)
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.EUTRANCGI, true },
	},
	{
		id: idTAI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TAI)
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.TAI, true },
	},
}

func (m *HandoverNotify) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverNotifyIEs, m)
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
	return parseMessageBody[HandoverNotify](ProcHandoverNotification, handoverNotifyIEs, value)
}
