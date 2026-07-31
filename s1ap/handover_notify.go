// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.5.7.
type HandoverNotify struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	EUTRANCGI   *EUTRANCGI
	TAI         *TAI

	messageMeta
}

var handoverNotifyIEs = []ieSpec[HandoverNotify]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idEUTRANCGI, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			var v EUTRANCGI

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.EUTRANCGI = &v

			return nil
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) {
			if m.EUTRANCGI == nil {
				return nil, false
			}

			return m.EUTRANCGI, true
		},
	},
	{
		id: idTAI, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverNotify, raw []byte, enc per.Encoding) error {
			var v TAI

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.TAI = &v

			return nil
		},
		encode: func(m *HandoverNotify) (per.Marshaler, bool) {
			if m.TAI == nil {
				return nil, false
			}

			return m.TAI, true
		},
	},
}

func (m *HandoverNotify) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverNotification, handoverNotifyIEs, m)
}

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

func ParseHandoverNotify(value []byte) (*HandoverNotify, error) {
	return parseMessageBody[HandoverNotify](ProcHandoverNotification, TriggeringInitiatingMessage, handoverNotifyIEs, value)
}
