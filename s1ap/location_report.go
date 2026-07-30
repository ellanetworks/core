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

// TS 36.413 §9.1.12.3.
type LocationReport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	EUTRANCGI   *EUTRANCGI
	TAI         *TAI
	RequestType *RequestType

	messageMeta
}

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
			var v EUTRANCGI

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.EUTRANCGI = &v

			return nil
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.EUTRANCGI == nil {
				return nil, false
			}

			return m.EUTRANCGI, true
		},
	},
	{
		id: idTAI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			var v TAI

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.TAI = &v

			return nil
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.TAI == nil {
				return nil, false
			}

			return m.TAI, true
		},
	},
	{
		id: idRequestType, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			var v RequestType

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RequestType = &v

			return nil
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.RequestType == nil {
				return nil, false
			}

			return m.RequestType, true
		},
	},
}

func (m *LocationReport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcLocationReport, locationReportIEs, m)
}

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

func ParseLocationReport(value []byte) (*LocationReport, error) {
	return parseMessageBody[LocationReport](ProcLocationReport, TriggeringInitiatingMessage, locationReportIEs, value)
}
