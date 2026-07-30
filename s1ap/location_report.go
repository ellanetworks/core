// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
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

// locationReportIEs is the LocationReport IE table (TS 36.413).
var locationReportIEs = []ieSpec[LocationReport]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idEUTRANCGI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.EUTRANCGI)
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.EUTRANCGI, true },
	},
	{
		id: idTAI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TAI)
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.TAI, true },
	},
	{
		id: idRequestType, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RequestType)
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.RequestType, true },
	},
}

func (m *LocationReport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, locationReportIEs, m)
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
	return parseMessageBody[LocationReport](ProcLocationReport, locationReportIEs, value)
}
