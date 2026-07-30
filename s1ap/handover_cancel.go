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
	Cause       Cause

	unmodeledIEs
}

var handoverCancelIEs = []ieSpec[HandoverCancel]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCancel, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCancel, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverCancel, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.Cause, true },
	},
}

func (m *HandoverCancel) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverCancelIEs, m)
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
	return parseMessageBody[HandoverCancel](ProcHandoverCancel, handoverCancelIEs, value)
}

// TS 36.413 §9.1.5.12.
type HandoverCancelAcknowledge struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID

	unmodeledIEs
}

var handoverCancelAcknowledgeIEs = []ieSpec[HandoverCancelAcknowledge]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverCancelAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverCancelAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
}

func (m *HandoverCancelAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverCancelAcknowledgeIEs, m)
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
	return parseMessageBody[HandoverCancelAcknowledge](ProcHandoverCancel, handoverCancelAcknowledgeIEs, value)
}
