// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.7.4.
type NASNonDeliveryIndication struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU
	Cause       *Cause

	messageMeta
}

var nASNonDeliveryIndicationIEs = []ieSpec[NASNonDeliveryIndication]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *NASNonDeliveryIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *NASNonDeliveryIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *NASNonDeliveryIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *NASNonDeliveryIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDNASPDU, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *NASNonDeliveryIndication, r *per.Reader, enc per.Encoding) error {
			if err := decodeIE(r, &m.NASPDU); err != nil {
				return err
			}

			if m.NASPDU == nil {
				m.NASPDU = NASPDU{}
			}

			return nil
		},
		Encode: func(m *NASNonDeliveryIndication) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *NASNonDeliveryIndication, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *NASNonDeliveryIndication) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
}

func (m *NASNonDeliveryIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcNASNonDeliveryIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func (m *NASNonDeliveryIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcNASNonDeliveryIndication, nASNonDeliveryIndicationIEs, m)
}

func ParseNASNonDeliveryIndication(value []byte) (*NASNonDeliveryIndication, error) {
	return parseMessageBody[NASNonDeliveryIndication](ProcNASNonDeliveryIndication, TriggeringInitiatingMessage, nASNonDeliveryIndicationIEs, value)
}

func DecodeNASNonDeliveryIndication(value []byte) (*NASNonDeliveryIndication, error) {
	return decodeMessageBody[NASNonDeliveryIndication](ProcNASNonDeliveryIndication, nASNonDeliveryIndicationIEs, value)
}
