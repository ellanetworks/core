// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.3.11. The AMF and the NG-RAN node cancel a prepared
// handover. TS 36.413 §9.1.5.11 carries the same three IEs.
type HandoverCancel struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	Cause       *Cause

	messageMeta
}

var handoverCancelIEs = []ieSpec[HandoverCancel]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCancel, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCancel, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
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

// TS 38.413 §9.2.3.12. TS 36.413 §9.1.5.12 is identical.
type HandoverCancelAcknowledge struct {
	AMFUENGAPID            *AMFUENGAPID
	RANUENGAPID            *RANUENGAPID
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var handoverCancelAcknowledgeIEs = []ieSpec[HandoverCancelAcknowledge]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverCancelAcknowledge, raw []byte, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverCancelAcknowledge, raw []byte, enc per.Encoding) error {
			var v RANUENGAPID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverCancelAcknowledge, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
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
