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
	extensions                per.ExtensionAdditions `per:",extraw"`
	EventType                 EventType
	ReportArea                ReportArea
	RequestTypeAdditionalInfo *RequestTypeAdditionalInfo `per:",extie:IDRequestTypeAdditionalInfo,crit:ignore"`
	iEExtensions              extensionMeta              `per:",exttable"`
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
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDEUTRANCGI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			var v EUTRANCGI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.EUTRANCGI = &v

			return nil
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.EUTRANCGI == nil {
				return nil, false
			}

			return m.EUTRANCGI, true
		},
	},
	{
		ID: IDTAI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			var v TAI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TAI = &v

			return nil
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.TAI == nil {
				return nil, false
			}

			return m.TAI, true
		},
	},
	{
		ID: IDRequestType, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			var v RequestType

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RequestType = &v

			return nil
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) {
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

func DecodeLocationReport(value []byte) (*LocationReport, error) {
	return decodeMessageBody[LocationReport](ProcLocationReport, locationReportIEs, value)
}
