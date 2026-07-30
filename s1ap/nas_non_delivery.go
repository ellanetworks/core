// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
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
	w.WriteBit(false)

	fields := []ieField{
		// Assigned criticalities per TS 36.413 §9.1.7.4: MME/eNB-UE-S1AP-ID reject,
		// NAS-PDU and Cause ignore.
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idNASPDU, crit: CriticalityIgnore, val: &m.NASPDU},
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// ParseNASNonDeliveryIndication decodes a NAS NON DELIVERY INDICATION (TS 36.413).
func ParseNASNonDeliveryIndication(value []byte) (*NASNonDeliveryIndication, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: NASNonDeliveryIndication preamble: %w", err)
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

	m := &NASNonDeliveryIndication{}

	var seenMME, seenENB, seenNAS, seenCause bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idNASPDU:
			err = perIEDecode(f.value, &m.NASPDU)
			seenNAS = true
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: NASNonDeliveryIndication IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcNASNonDeliveryIndication,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idNASPDU, CriticalityIgnore, seenNAS},
		ieCheck{idCause, CriticalityIgnore, seenCause},
	); err != nil {
		return nil, err
	}

	return m, nil
}
