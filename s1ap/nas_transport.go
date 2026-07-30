// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// InitialUEMessage is the INITIAL UE MESSAGE (TS 36.413), sent by the
// eNB to deliver a UE's first NAS message. Unmodeled IEs are preserved.
type InitialUEMessage struct {
	ENBUES1APID           ENBUES1APID
	NASPDU                NASPDU
	TAI                   TAI
	EUTRANCGI             EUTRANCGI
	RRCEstablishmentCause RRCEstablishmentCause
	STMSI                 *STMSI  // present when the UE re-establishes with an S-TMSI
	GUMMEI                *GUMMEI // the eNB-selected MME, present when the eNB does not run NNSF

	unmodeledIEs
}

func (m *InitialUEMessage) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idNASPDU, crit: CriticalityReject, val: &m.NASPDU},
		{id: idTAI, crit: CriticalityReject, val: &m.TAI},
		{id: idEUTRANCGI, crit: CriticalityIgnore, val: &m.EUTRANCGI},
		{id: idRRCEstablishmentCause, crit: CriticalityIgnore, val: &m.RRCEstablishmentCause},
	}

	if m.STMSI != nil {
		fields = append(fields, ieField{id: idSTMSI, crit: CriticalityReject, val: m.STMSI})
	}

	if m.GUMMEI != nil {
		fields = append(fields, ieField{id: idGUMMEI, crit: CriticalityReject, val: m.GUMMEI})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialUEMessage) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialUEMessage,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseInitialUEMessage decodes an InitialUEMessage from the open-type payload
// of an initiatingMessage.
func ParseInitialUEMessage(value []byte) (*InitialUEMessage, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: InitialUEMessage preamble: %w", err)
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

	m := &InitialUEMessage{}

	var seenENB, seenNAS, seenTAI, seenCGI, seenRRC bool

	for _, f := range fields {
		switch f.id {
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idNASPDU:
			err = perIEDecode(f.value, &m.NASPDU)
			seenNAS = true
		case idTAI:
			err = perIEDecode(f.value, &m.TAI)
			seenTAI = true
		case idEUTRANCGI:
			err = perIEDecode(f.value, &m.EUTRANCGI)
			seenCGI = true
		case idRRCEstablishmentCause:
			err = perIEDecode(f.value, &m.RRCEstablishmentCause)
			seenRRC = true
		case idSTMSI:
			var stmsi STMSI

			err = perIEDecode(f.value, &stmsi)
			m.STMSI = &stmsi
		case idGUMMEI:
			var gummei GUMMEI

			err = perIEDecode(f.value, &gummei)
			m.GUMMEI = &gummei
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: InitialUEMessage IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcInitialUEMessage,
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idNASPDU, CriticalityReject, seenNAS},
		ieCheck{idTAI, CriticalityReject, seenTAI},
		ieCheck{idEUTRANCGI, CriticalityIgnore, seenCGI},
		ieCheck{idRRCEstablishmentCause, CriticalityIgnore, seenRRC},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// UplinkNASTransport is the UPLINK NAS TRANSPORT message (TS 36.413),
// sent by the eNB to relay a UE's NAS message on an established UE context.
type UplinkNASTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU
	EUTRANCGI   EUTRANCGI
	TAI         TAI

	unmodeledIEs
}

func (m *UplinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idNASPDU, crit: CriticalityReject, val: &m.NASPDU},
		{id: idEUTRANCGI, crit: CriticalityIgnore, val: &m.EUTRANCGI},
		{id: idTAI, crit: CriticalityIgnore, val: &m.TAI},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UplinkNASTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkNASTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseUplinkNASTransport decodes an UplinkNASTransport from the open-type
// payload of an initiatingMessage.
func ParseUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: UplinkNASTransport preamble: %w", err)
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

	m := &UplinkNASTransport{}

	var seenMME, seenENB, seenNAS, seenCGI, seenTAI bool

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
		case idEUTRANCGI:
			err = perIEDecode(f.value, &m.EUTRANCGI)
			seenCGI = true
		case idTAI:
			err = perIEDecode(f.value, &m.TAI)
			seenTAI = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UplinkNASTransport IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcUplinkNASTransport,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idNASPDU, CriticalityReject, seenNAS},
		ieCheck{idEUTRANCGI, CriticalityIgnore, seenCGI},
		ieCheck{idTAI, CriticalityIgnore, seenTAI},
	); err != nil {
		return nil, err
	}

	return m, nil
}

// DownlinkNASTransport is the DOWNLINK NAS TRANSPORT message (TS 36.413),
// sent by the MME to relay a NAS message to the UE.
type DownlinkNASTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU

	unmodeledIEs
}

func (m *DownlinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idNASPDU, crit: CriticalityReject, val: &m.NASPDU},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *DownlinkNASTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkNASTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseDownlinkNASTransport decodes a DownlinkNASTransport from the open-type
// payload of an initiatingMessage.
func ParseDownlinkNASTransport(value []byte) (*DownlinkNASTransport, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: DownlinkNASTransport preamble: %w", err)
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

	m := &DownlinkNASTransport{}

	var seenMME, seenENB, seenNAS bool

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
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: DownlinkNASTransport IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcDownlinkNASTransport,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idNASPDU, CriticalityReject, seenNAS},
	); err != nil {
		return nil, err
	}

	return m, nil
}
