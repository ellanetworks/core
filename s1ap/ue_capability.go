// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.10.
type UECapabilityInfoIndication struct {
	MMEUES1APID                MMEUES1APID
	ENBUES1APID                ENBUES1APID
	UERadioCapability          []byte
	UERadioCapabilityForPaging []byte // paging-specific capability (TS 36.413), when present
	messageMeta
}

var uECapabilityInfoIndicationIEs = []ieSpec[UECapabilityInfoIndication]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idUERadioCapability, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.UERadioCapability, err = per.DecodeOctetString(per.NewReader(raw), enc, 0, 0, true, false, false)

			return err
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapability == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapability)
			}), true
		},
	},
	{
		id: idUERadioCapabilityForPaging, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.UERadioCapabilityForPaging, err = per.DecodeOctetString(per.NewReader(raw), enc, 0, 0, true, false, false)

			return err
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapabilityForPaging)
			}), true
		},
	},
}

func (m *UECapabilityInfoIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUECapabilityInfoIndication, uECapabilityInfoIndicationIEs, m)
}

func (m *UECapabilityInfoIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUECapabilityInfoIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUECapabilityInfoIndication(value []byte) (*UECapabilityInfoIndication, error) {
	return parseMessageBody[UECapabilityInfoIndication](ProcUECapabilityInfoIndication, TriggeringInitiatingMessage, uECapabilityInfoIndicationIEs, value)
}
