// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// UECapabilityInfoIndication is the UE CAPABILITY INFO INDICATION message
// (TS 36.413), sent by the eNB to give the MME the UE's radio capability.
// Only the fields the MME consumes are modelled; the UE Radio Capability is an
// OCTET STRING carried opaquely (the MME stores it and replays it in the INITIAL
// CONTEXT SETUP REQUEST per TS 23.401).
type UECapabilityInfoIndication struct {
	MMEUES1APID                MMEUES1APID
	ENBUES1APID                ENBUES1APID
	UERadioCapability          []byte
	UERadioCapabilityForPaging []byte // paging-specific capability (TS 36.413), when present
	unmodeledIEs
}

// uECapabilityInfoIndicationIEs is the UECapabilityInfoIndication IE table (TS 36.413).
var uECapabilityInfoIndicationIEs = []ieSpec[UECapabilityInfoIndication]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idUERadioCapability, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UECapabilityInfoIndication, raw []byte, enc per.Encoding) error {
			var err error

			m.UERadioCapability, err = per.DecodeOctetString(per.NewReader(raw), enc, 0, 0, true, false, false)

			return err
		},
		encode: func(m *UECapabilityInfoIndication) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapability)
			}), true
		},
	},
	{
		id: idUERadioCapabilityForPaging, presence: PresenceOptional, crit: CriticalityIgnore,
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
	return encodeMessageBody(w, enc, uECapabilityInfoIndicationIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseUECapabilityInfoIndication decodes the message from an initiatingMessage
// open-type payload.
func ParseUECapabilityInfoIndication(value []byte) (*UECapabilityInfoIndication, error) {
	return parseMessageBody[UECapabilityInfoIndication](ProcUECapabilityInfoIndication, uECapabilityInfoIndicationIEs, value)
}
