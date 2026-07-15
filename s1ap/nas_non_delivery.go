// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

// NASNonDeliveryIndication reports a downlink NAS-PDU the eNB could not deliver to
// the UE (TS 36.413 §9.1.7.4). All four IEs are mandatory. The NAS-PDU is the
// undelivered downlink message, carried for diagnostics only — it must not be
// reprocessed as uplink.
type NASNonDeliveryIndication struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU
	Cause       Cause

	unmodeledIEs
}

// Marshal encodes the NAS NON DELIVERY INDICATION as an initiating message (TS 36.413).
func (m *NASNonDeliveryIndication) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcNASNonDeliveryIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func (m *NASNonDeliveryIndication) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		// Assigned criticalities per TS 36.413 §9.1.7.4: MME/eNB-UE-S1AP-ID reject,
		// NAS-PDU and Cause ignore.
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idNASPDU, crit: CriticalityIgnore, enc: m.NASPDU.encode},
		{id: idCause, crit: CriticalityIgnore, enc: m.Cause.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// ParseNASNonDeliveryIndication decodes a NAS NON DELIVERY INDICATION (TS 36.413).
func ParseNASNonDeliveryIndication(value []byte) (*NASNonDeliveryIndication, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: NASNonDeliveryIndication preamble: %w", err)
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

	m := &NASNonDeliveryIndication{}

	var seenMME, seenENB, seenNAS, seenCause bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idNASPDU:
			m.NASPDU, err = decodeNASPDU(sub)
			seenNAS = true
		case idCause:
			m.Cause, err = decodeCause(sub)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: NASNonDeliveryIndication IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenNAS || !seenCause {
		return nil, fmt.Errorf("s1ap: NASNonDeliveryIndication missing mandatory IE")
	}

	return m, nil
}
