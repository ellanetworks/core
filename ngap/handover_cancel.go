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
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverCancel, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverCancel, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *HandoverCancel) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverCancel, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *HandoverCancel) (per.Marshaler, bool) {
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

func DecodeHandoverCancel(value []byte) (*HandoverCancel, error) {
	return decodeMessageBody[HandoverCancel](ProcHandoverCancel, handoverCancelIEs, value)
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
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverCancelAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v AMFUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFUENGAPID = &v

			return nil
		},
		Encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) {
			if m.AMFUENGAPID == nil {
				return nil, false
			}

			return m.AMFUENGAPID, true
		},
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverCancelAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v RANUENGAPID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RANUENGAPID = &v

			return nil
		},
		Encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) {
			if m.RANUENGAPID == nil {
				return nil, false
			}

			return m.RANUENGAPID, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverCancelAcknowledge, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *HandoverCancelAcknowledge) (per.Marshaler, bool) {
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

func DecodeHandoverCancelAcknowledge(value []byte) (*HandoverCancelAcknowledge, error) {
	return decodeMessageBody[HandoverCancelAcknowledge](ProcHandoverCancel, handoverCancelAcknowledgeIEs, value)
}
