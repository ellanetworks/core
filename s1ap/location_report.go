// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// EventType ::= ENUMERATED { direct, change-of-serve-cell, stop-change-of-serve-cell,
// ... } (TS 36.413).
type EventType uint8

const (
	EventTypeDirect EventType = iota
	EventTypeChangeOfServeCell
	EventTypeStopChangeOfServeCell

	eventTypeRootCount = 3
)

// ReportArea ::= ENUMERATED { ecgi, ... } (TS 36.413).
type ReportArea uint8

const (
	ReportAreaECGI ReportArea = iota

	reportAreaRootCount = 1
)

// RequestType ::= SEQUENCE { eventType, reportArea, iE-Extensions OPTIONAL, ... }
// (TS 36.413 §9.2.1.35).
type RequestType struct {
	EventType  EventType
	ReportArea ReportArea
}

func (rt RequestType) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := w.WriteEnum(int(rt.EventType), eventTypeRootCount, true, false); err != nil {
		return err
	}

	return w.WriteEnum(int(rt.ReportArea), reportAreaRootCount, true, false)
}

func decodeRequestType(r *aper.Reader) (RequestType, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return RequestType{}, err
	}

	et, _, err := r.ReadEnum(eventTypeRootCount, true)
	if err != nil {
		return RequestType{}, err
	}

	ra, _, err := r.ReadEnum(reportAreaRootCount, true)
	if err != nil {
		return RequestType{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return RequestType{}, err
	}

	return RequestType{EventType: EventType(et), ReportArea: ReportArea(ra)}, nil
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

func (m *LocationReport) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: m.MMEUES1APID.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: m.ENBUES1APID.encode},
		{id: idEUTRANCGI, crit: CriticalityIgnore, enc: m.EUTRANCGI.encode},
		{id: idTAI, crit: CriticalityIgnore, enc: m.TAI.encode},
		{id: idRequestType, crit: CriticalityIgnore, enc: m.RequestType.encode},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *LocationReport) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcLocationReport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

// ParseLocationReport decodes a LocationReport from the open-type payload of an
// initiatingMessage.
func ParseLocationReport(value []byte) (*LocationReport, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: LocationReport preamble: %w", err)
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

	m := &LocationReport{}

	var seenMME, seenENB, seenCGI, seenTAI, seenReq bool

	for _, f := range fields {
		sub := aper.NewReader(f.value)

		switch f.id {
		case idMMEUES1APID:
			m.MMEUES1APID, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			m.ENBUES1APID, err = decodeENBUES1APID(sub)
			seenENB = true
		case idEUTRANCGI:
			m.EUTRANCGI, err = decodeEUTRANCGI(sub)
			seenCGI = true
		case idTAI:
			m.TAI, err = decodeTAI(sub)
			seenTAI = true
		case idRequestType:
			m.RequestType, err = decodeRequestType(sub)
			seenReq = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: LocationReport IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenCGI || !seenTAI || !seenReq {
		return nil, fmt.Errorf("s1ap: LocationReport missing mandatory IE")
	}

	return m, nil
}
