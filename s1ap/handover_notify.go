// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.5.7.
type HandoverNotify struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	EUTRANCGI               *EUTRANCGI
	TAI                     *TAI
	TunnelInformationForBBF *TunnelInformation

	messageMeta
}

var handoverNotifyIEs = []ieSpec[HandoverNotify]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDEUTRANCGI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			var v EUTRANCGI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.EUTRANCGI = &v

			return nil
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) {
			if m.EUTRANCGI == nil {
				return nil, false
			}

			return m.EUTRANCGI, true
		},
	},
	{
		ID: IDTAI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			var v TAI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TAI = &v

			return nil
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) {
			if m.TAI == nil {
				return nil, false
			}

			return m.TAI, true
		},
	},
	{
		ID: IDTunnelInformationForBBF, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverNotify, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.TunnelInformationForBBF)
		},
		Encode: func(m *HandoverNotify) (per.Marshaler, bool) { return optionalIE(m.TunnelInformationForBBF) },
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

func DecodeHandoverNotify(value []byte) (*HandoverNotify, error) {
	return decodeMessageBody[HandoverNotify](ProcHandoverNotification, handoverNotifyIEs, value)
}
