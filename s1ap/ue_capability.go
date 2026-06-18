// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// UECapabilityInfoIndication is the UE CAPABILITY INFO INDICATION message
// (TS 36.413 §8.9), sent by the eNB to give the MME the UE's radio capability.
// Only the fields the MME consumes are modelled; the UE Radio Capability is an
// OCTET STRING carried opaquely (the MME stores it and replays it in the INITIAL
// CONTEXT SETUP REQUEST per TS 23.401 §5.11.2).
type UECapabilityInfoIndication struct {
	MMEUES1APID                MMEUES1APID
	ENBUES1APID                ENBUES1APID
	UERadioCapability          []byte
	UERadioCapabilityForPaging []byte // paging-specific capability (TS 36.413 §9.2.1.98), when present
	unmodeledIEs
}

func (m *UECapabilityInfoIndication) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idUERadioCapability, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return w.WriteOctetString(m.UERadioCapability, 0, aper.Unbounded, false)
		}},
	}

	if m.UERadioCapabilityForPaging != nil {
		fields = append(fields, ieField{id: idUERadioCapabilityForPaging, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
			return w.WriteOctetString(m.UERadioCapabilityForPaging, 0, aper.Unbounded, false)
		}})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UECapabilityInfoIndication) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUECapabilityInfoIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseUECapabilityInfoIndication decodes the message from an initiatingMessage
// open-type payload.
func ParseUECapabilityInfoIndication(value []byte) (*UECapabilityInfoIndication, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: UECapabilityInfoIndication preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &UECapabilityInfoIndication{}

	var seenMME, seenENB, seenCap bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idUERadioCapability:
			m.UERadioCapability, err = sub.ReadOctetString(0, aper.Unbounded, false)
			seenCap = true
		case idUERadioCapabilityForPaging:
			m.UERadioCapabilityForPaging, err = sub.ReadOctetString(0, aper.Unbounded, false)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UECapabilityInfoIndication IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenCap {
		return nil, fmt.Errorf("s1ap: UECapabilityInfoIndication missing mandatory IE")
	}

	return m, nil
}
