// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.13.1.
type UERadioCapabilityInfoIndication struct {
	AMFUENGAPID       AMFUENGAPID
	RANUENGAPID       RANUENGAPID
	UERadioCapability UERadioCapability
	// A SEQUENCE of separate NR and E-UTRA capabilities, where S1AP's
	// counterpart is one opaque OCTET STRING (§9.3.1.68).
	UERadioCapabilityForPaging *UERadioCapabilityForPaging

	messageMeta
}

var uERadioCapabilityInfoIndicationIEs = []ieSpec[UERadioCapabilityInfoIndication]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UERadioCapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *UERadioCapabilityInfoIndication) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UERadioCapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *UERadioCapabilityInfoIndication) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idUERadioCapability, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UERadioCapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UERadioCapability)
		},
		encode: func(m *UERadioCapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapability == nil {
				return nil, false
			}

			return m.UERadioCapability, true
		},
	},
	{
		id: idUERadioCapabilityForPaging, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UERadioCapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			var v UERadioCapabilityForPaging

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UERadioCapabilityForPaging = &v

			return nil
		},
		encode: func(m *UERadioCapabilityInfoIndication) (per.Marshaler, bool) {
			if m.UERadioCapabilityForPaging == nil {
				return nil, false
			}

			return m.UERadioCapabilityForPaging, true
		},
	},
}

func (m *UERadioCapabilityInfoIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUERadioCapabilityInfoIndication, uERadioCapabilityInfoIndicationIEs, m)
}

func (m *UERadioCapabilityInfoIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUERadioCapabilityInfoIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUERadioCapabilityInfoIndication(value []byte) (*UERadioCapabilityInfoIndication, error) {
	return parseMessageBody[UERadioCapabilityInfoIndication](ProcUERadioCapabilityInfoIndication, TriggeringInitiatingMessage, uERadioCapabilityInfoIndicationIEs, value)
}
