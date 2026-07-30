// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// HandoverCancel is the HANDOVER CANCEL message (TS 36.413), sent by
// the source eNB to cancel an ongoing or prepared handover (TS 23.401).
type HandoverCancel struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       Cause

	unmodeledIEs
}

// handoverCancelIEs is the HandoverCancel IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseHandoverCancel decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverCancel(value []byte) (*HandoverCancel, error) {
	return parseMessageBody[HandoverCancel](ProcHandoverCancel, handoverCancelIEs, value)
}

// HandoverCancelAcknowledge is the HANDOVER CANCEL ACKNOWLEDGE message (TS 36.413),
// the successful outcome the MME returns to confirm the handover has
// been cancelled and target resources released.
type HandoverCancelAcknowledge struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID

	unmodeledIEs
}

// handoverCancelAcknowledgeIEs is the HandoverCancelAcknowledge IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseHandoverCancelAcknowledge decodes the message from a successfulOutcome
// open-type payload.
func ParseHandoverCancelAcknowledge(value []byte) (*HandoverCancelAcknowledge, error) {
	return parseMessageBody[HandoverCancelAcknowledge](ProcHandoverCancel, handoverCancelAcknowledgeIEs, value)
}
