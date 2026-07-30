// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

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

func (m *UECapabilityInfoIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idUERadioCapability, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapability)
		})},
	}

	if m.UERadioCapabilityForPaging != nil {
		fields = append(fields, ieField{id: idUERadioCapabilityForPaging, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return per.EncodeOctetString(w, enc, 0, 0, true, false, false, m.UERadioCapabilityForPaging)
		})})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
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
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: UECapabilityInfoIndication preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &UECapabilityInfoIndication{}

	var seenMME, seenENB, seenCap bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idUERadioCapability:
			m.UERadioCapability, err = per.DecodeOctetString(per.NewReader(f.value), enc, 0, 0, true, false, false)
			seenCap = true
		case idUERadioCapabilityForPaging:
			m.UERadioCapabilityForPaging, err = per.DecodeOctetString(per.NewReader(f.value), enc, 0, 0, true, false, false)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UECapabilityInfoIndication IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcUECapabilityInfoIndication,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idUERadioCapability, CriticalityIgnore, seenCap},
	); err != nil {
		return nil, err
	}

	return m, nil
}
