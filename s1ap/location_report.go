// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// EventType ::= ENUMERATED { direct, change-of-serve-cell, stop-change-of-serve-cell,
// ... } (TS 36.413).
type EventType uint8

const (
	EventTypeDirect EventType = iota
	EventTypeChangeOfServeCell
	EventTypeStopChangeOfServeCell
)

// ReportArea ::= ENUMERATED { ecgi, ... } (TS 36.413).
type ReportArea uint8

const (
	ReportAreaECGI ReportArea = iota
)

// RequestType ::= SEQUENCE { eventType, reportArea, iE-Extensions OPTIONAL, ... }
// (TS 36.413 §9.2.1.35).
type RequestType struct {
	_          [0]struct{}  `per:"extseq"`
	EventType  EventType    `per:"ENUMERATED,range:0..2,..."`
	ReportArea ReportArea   `per:"ENUMERATED,range:0..0,..."`
	_          ieExtensions `per:",skip"`
}

// LocationReport is the LOCATION REPORT message (TS 36.413), sent by the eNB to
// report the UE's serving cell.
type LocationReport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	EUTRANCGI   EUTRANCGI
	TAI         TAI
	RequestType RequestType

	unmodeledIEs
}

func (m *LocationReport) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityReject, val: &m.ENBUES1APID},
		{id: idEUTRANCGI, crit: CriticalityIgnore, val: &m.EUTRANCGI},
		{id: idTAI, crit: CriticalityIgnore, val: &m.TAI},
		{id: idRequestType, crit: CriticalityIgnore, val: &m.RequestType},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *LocationReport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcLocationReport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseLocationReport decodes a LocationReport from the open-type payload of an
// initiatingMessage.
func ParseLocationReport(value []byte) (*LocationReport, error) {
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: LocationReport preamble: %w", err)
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

	m := &LocationReport{}

	var seenMME, seenENB, seenCGI, seenTAI, seenReq bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idEUTRANCGI:
			err = perIEDecode(f.value, &m.EUTRANCGI)
			seenCGI = true
		case idTAI:
			err = perIEDecode(f.value, &m.TAI)
			seenTAI = true
		case idRequestType:
			err = perIEDecode(f.value, &m.RequestType)
			seenReq = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: LocationReport IE %d: %w", f.id, err)
		}
	}

	if err := requireIEs(ProcLocationReport,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idEUTRANCGI, CriticalityIgnore, seenCGI},
		ieCheck{idTAI, CriticalityIgnore, seenTAI},
		ieCheck{idRequestType, CriticalityIgnore, seenReq},
	); err != nil {
		return nil, err
	}

	return m, nil
}
