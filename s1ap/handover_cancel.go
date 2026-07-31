// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.5.11.
type HandoverCancel struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       *Cause

	messageMeta
}

var handoverCancelIEs = []ieSpec[HandoverCancel]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCancel, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCancel, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverCancel, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverCancel) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
}

func (m *HandoverCancel) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverCancel, handoverCancelIEs, m)
}

func (m *HandoverCancel) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverCancel,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverCancel(value []byte) (*HandoverCancel, error) {
	return parseMessageBody[HandoverCancel](ProcHandoverCancel, TriggeringInitiatingMessage, handoverCancelIEs, value)
}

// TS 36.413 §9.1.5.12.
type HandoverCancelAcknowledge struct {
	MMEUES1APID *MMEUES1APID
	ENBUES1APID *ENBUES1APID

	messageMeta
}

var handoverCancelAcknowledgeIEs = []ieSpec[HandoverCancelAcknowledge]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverCancelAcknowledge, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverCancelAcknowledge, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
}

func (m *HandoverCancelAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverCancel, handoverCancelAcknowledgeIEs, m)
}

func (m *HandoverCancelAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverCancel,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverCancelAcknowledge(value []byte) (*HandoverCancelAcknowledge, error) {
	return parseMessageBody[HandoverCancelAcknowledge](ProcHandoverCancel, TriggeringSuccessfulOutcome, handoverCancelAcknowledgeIEs, value)
}
