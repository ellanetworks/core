// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// InitialUEMessage is the INITIAL UE MESSAGE (TS 36.413 §9.1.7.1), sent by the
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

func (m *InitialUEMessage) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idNASPDU, crit: CriticalityReject, enc: m.NASPDU.encode},
		{id: idTAI, crit: CriticalityReject, enc: m.TAI.encode},
		{id: idEUTRANCGI, crit: CriticalityIgnore, enc: m.EUTRANCGI.encode},
		{id: idRRCEstablishmentCause, crit: CriticalityIgnore, enc: m.RRCEstablishmentCause.encode},
	}

	if m.STMSI != nil {
		fields = append(fields, ieField{id: idSTMSI, crit: CriticalityReject, enc: m.STMSI.encode})
	}

	if m.GUMMEI != nil {
		fields = append(fields, ieField{id: idGUMMEI, crit: CriticalityReject, enc: m.GUMMEI.encode})
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *InitialUEMessage) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialUEMessage,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseInitialUEMessage decodes an InitialUEMessage from the open-type payload
// of an initiatingMessage.
func ParseInitialUEMessage(value []byte) (*InitialUEMessage, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: InitialUEMessage preamble: %w", err)
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

	m := &InitialUEMessage{}

	var seenENB, seenNAS, seenTAI, seenCGI, seenRRC bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idNASPDU:
			m.NASPDU, err = decodeNASPDU(sub)
			seenNAS = true
		case idTAI:
			m.TAI, err = decodeTAI(sub)
			seenTAI = true
		case idEUTRANCGI:
			m.EUTRANCGI, err = decodeEUTRANCGI(sub)
			seenCGI = true
		case idRRCEstablishmentCause:
			m.RRCEstablishmentCause, err = decodeRRCEstablishmentCause(sub)
			seenRRC = true
		case idSTMSI:
			var stmsi STMSI

			stmsi, err = decodeSTMSI(sub)
			m.STMSI = &stmsi
		case idGUMMEI:
			var gummei GUMMEI

			gummei, err = decodeGUMMEI(sub)
			m.GUMMEI = &gummei
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: InitialUEMessage IE %d: %w", f.id, err)
		}
	}

	if !seenENB || !seenNAS || !seenTAI || !seenCGI || !seenRRC {
		return nil, fmt.Errorf("s1ap: InitialUEMessage missing mandatory IE")
	}

	return m, nil
}

// UplinkNASTransport is the UPLINK NAS TRANSPORT message (TS 36.413 §9.1.7.3),
// sent by the eNB to relay a UE's NAS message on an established UE context.
type UplinkNASTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU
	EUTRANCGI   EUTRANCGI
	TAI         TAI

	unmodeledIEs
}

func (m *UplinkNASTransport) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idNASPDU, crit: CriticalityReject, enc: m.NASPDU.encode},
		{id: idEUTRANCGI, crit: CriticalityIgnore, enc: m.EUTRANCGI.encode},
		{id: idTAI, crit: CriticalityIgnore, enc: m.TAI.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UplinkNASTransport) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkNASTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseUplinkNASTransport decodes an UplinkNASTransport from the open-type
// payload of an initiatingMessage.
func ParseUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: UplinkNASTransport preamble: %w", err)
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

	m := &UplinkNASTransport{}

	var seenMME, seenENB, seenNAS, seenCGI, seenTAI bool

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
		case idEUTRANCGI:
			m.EUTRANCGI, err = decodeEUTRANCGI(sub)
			seenCGI = true
		case idTAI:
			m.TAI, err = decodeTAI(sub)
			seenTAI = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: UplinkNASTransport IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenNAS || !seenCGI || !seenTAI {
		return nil, fmt.Errorf("s1ap: UplinkNASTransport missing mandatory IE")
	}

	return m, nil
}

// DownlinkNASTransport is the DOWNLINK NAS TRANSPORT message (TS 36.413
// §9.1.7.2), sent by the MME to relay a NAS message to the UE.
type DownlinkNASTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU

	unmodeledIEs
}

func (m *DownlinkNASTransport) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idNASPDU, crit: CriticalityReject, enc: m.NASPDU.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *DownlinkNASTransport) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkNASTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseDownlinkNASTransport decodes a DownlinkNASTransport from the open-type
// payload of an initiatingMessage.
func ParseDownlinkNASTransport(value []byte) (*DownlinkNASTransport, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: DownlinkNASTransport preamble: %w", err)
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

	m := &DownlinkNASTransport{}

	var seenMME, seenENB, seenNAS bool

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
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: DownlinkNASTransport IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenNAS {
		return nil, fmt.Errorf("s1ap: DownlinkNASTransport missing mandatory IE")
	}

	return m, nil
}
