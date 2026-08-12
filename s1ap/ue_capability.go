// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.10.
type UECapabilityInfoIndication struct {
	MMEUES1APID       MMEUES1APID
	ENBUES1APID       ENBUES1APID
	UERadioCapability UERadioCapability
	// One opaque OCTET STRING, where NGAP's counterpart is a SEQUENCE of
	// separate NR and E-UTRA capabilities (§9.2.1.98).
	UERadioCapabilityForPaging UERadioCapabilityForPaging

	messageMeta
}

var uECapabilityInfoIndicationIEs = []ieSpec[UECapabilityInfoIndication]{
	{
		id: IDMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: IDUERadioCapability, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UERadioCapability)
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapability == nil {
				return nil, false
			}

			return m.UERadioCapability, true
		},
	},
	{
		id: IDUERadioCapabilityForPaging, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UERadioCapabilityForPaging)
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
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
