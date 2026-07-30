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
	Cause       Cause

	unmodeledIEs
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

var nASNonDeliveryIndicationIEs = []ieSpec[NASNonDeliveryIndication]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *NASNonDeliveryIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *NASNonDeliveryIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *NASNonDeliveryIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *NASNonDeliveryIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idNASPDU, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *NASNonDeliveryIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASPDU)
		},
		encode: func(m *NASNonDeliveryIndication) (per.Marshaler, bool) { return &m.NASPDU, true },
	},
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *NASNonDeliveryIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *NASNonDeliveryIndication) (per.Marshaler, bool) { return &m.Cause, true },
	},
}

func (m *NASNonDeliveryIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, nASNonDeliveryIndicationIEs, m)
}

func ParseNASNonDeliveryIndication(value []byte) (*NASNonDeliveryIndication, error) {
	return parseMessageBody[NASNonDeliveryIndication](ProcNASNonDeliveryIndication, nASNonDeliveryIndicationIEs, value)
}
