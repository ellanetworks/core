// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// maxnoofAoI bounds the areas of interest one request may name.
const maxnoofAoI = 64

// EventType ::= ENUMERATED { direct, change-of-serve-cell,
// ue-presence-in-area-of-interest, stop-change-of-serve-cell,
// stop-ue-presence-in-area-of-interest, cancel-location-reporting-for-the-ue,
// ... } — TS 38.413 §9.3.1.32.
//
// TS 36.413's EventType has three root members, and the two enumerations agree
// only on direct and change-of-serve-cell: S1AP puts stop-change-of-serve-cell
// at index 2 where NGAP puts it at 3. A value cannot be carried across.
type EventType uint8

const (
	EventTypeDirect EventType = iota
	EventTypeChangeOfServeCell
	EventTypeUEPresenceInAreaOfInterest
	EventTypeStopChangeOfServeCell
	EventTypeStopUEPresenceInAreaOfInterest
	EventTypeCancelLocationReportingForTheUE

	eventTypeRootCount = 6
)

// ReportArea ::= ENUMERATED { cell, ... } — TS 38.413 §9.3.1.33. TS 36.413
// names its single member ecgi; the wire encoding is the same.
type ReportArea uint8

const (
	ReportAreaCell ReportArea = iota

	reportAreaRootCount = 1
)

// LocationReportingReferenceID ::= INTEGER (1..64, ...) — TS 38.413 §9.3.1.133.
type LocationReportingReferenceID uint8

func (id LocationReportingReferenceID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, maxnoofAoI, int64(id))
}

func (id *LocationReportingReferenceID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 1, maxnoofAoI, "LocationReportingReferenceID")
	if err != nil {
		return err
	}

	*id = LocationReportingReferenceID(v)

	return nil
}

// LocationReportingRequestType ::= SEQUENCE { eventType, reportArea,
// areaOfInterestList OPTIONAL, locationReportingReferenceIDToBeCancelled
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.65.
// TS 36.413 §9.2.1.35 names the same shape RequestType and has neither optional
// field.
//
// The codec is hand-written because areaOfInterestList is deliberately not
// modelled: this AMF never asks for area-of-interest reporting, so a peer has
// no reason to name one back. Its preamble bit is still read, and a request
// that sets it is refused rather than silently misread.
type LocationReportingRequestType struct {
	EventType                                 EventType
	ReportArea                                ReportArea
	LocationReportingReferenceIDToBeCancelled *LocationReportingReferenceID
}

func (t LocationReportingRequestType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)
	// areaOfInterestList, which this AMF never sends.
	w.WriteBit(false)
	w.WriteBit(t.LocationReportingReferenceIDToBeCancelled != nil)
	// iE-Extensions.
	w.WriteBit(false)

	if err := encodeRootEnumerated(w, enc, eventTypeRootCount, int64(t.EventType), "EventType"); err != nil {
		return err
	}

	if err := encodeRootEnumerated(w, enc, reportAreaRootCount, int64(t.ReportArea), "ReportArea"); err != nil {
		return err
	}

	if t.LocationReportingReferenceIDToBeCancelled != nil {
		return t.LocationReportingReferenceIDToBeCancelled.MarshalPER(w, enc)
	}

	return nil
}

func (t *LocationReportingRequestType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extPresent, err := r.ReadBit()
	if err != nil {
		return err
	}

	haveAreaOfInterest, err := r.ReadBit()
	if err != nil {
		return err
	}

	haveReferenceID, err := r.ReadBit()
	if err != nil {
		return err
	}

	haveExtensions, err := r.ReadBit()
	if err != nil {
		return err
	}

	eventType, err := decodeRootEnumerated(r, enc, eventTypeRootCount, "EventType")
	if err != nil {
		return err
	}

	t.EventType = EventType(eventType)

	reportArea, err := decodeRootEnumerated(r, enc, reportAreaRootCount, "ReportArea")
	if err != nil {
		return err
	}

	t.ReportArea = ReportArea(reportArea)

	if haveAreaOfInterest {
		return fmt.Errorf("%w: LocationReportingRequestType areaOfInterestList", errNotComprehended)
	}

	if haveReferenceID {
		var id LocationReportingReferenceID

		if err := id.UnmarshalPER(r, enc); err != nil {
			return err
		}

		t.LocationReportingReferenceIDToBeCancelled = &id
	}

	if haveExtensions {
		if err := skipSequenceExtensionsPER(r, enc, true, false); err != nil {
			return err
		}
	}

	if extPresent {
		return skipSequenceExtensionsPER(r, enc, false, true)
	}

	return nil
}

// UEPresence ::= ENUMERATED { in, out, unknown, ... } — TS 38.413 §9.3.1.66.
type UEPresence uint8

const (
	UEPresenceIn UEPresence = iota
	UEPresenceOut
	UEPresenceUnknown
)

// UEPresenceInAreaOfInterestItem ::= SEQUENCE { locationReportingReferenceID,
// uEPresence, iE-Extensions OPTIONAL } (extensible).
type UEPresenceInAreaOfInterestItem struct {
	_                            [0]struct{} `per:"extseq"`
	LocationReportingReferenceID LocationReportingReferenceID
	UEPresence                   UEPresence   `per:"ENUMERATED,range:0..2,..."`
	_                            ieExtensions `per:",skip"`
}

// UEPresenceInAreaOfInterestList ::= SEQUENCE (SIZE(1..maxnoofAoI)) OF
// UEPresenceInAreaOfInterestItem.
type UEPresenceInAreaOfInterestList []UEPresenceInAreaOfInterestItem

// TS 38.413 §9.2.8.3. The NG-RAN node reports where the UE is. TS 36.413
// §9.1.8.3 splits the location into EUTRAN-CGI and TAI where NGAP carries one
// UserLocationInformation, and has no UE-presence list because S1AP defines no
// area of interest.
type LocationReport struct {
	AMFUENGAPID                    AMFUENGAPID
	RANUENGAPID                    RANUENGAPID
	UserLocationInformation        *UserLocationInformation
	UEPresenceInAreaOfInterestList UEPresenceInAreaOfInterestList
	LocationReportingRequestType   *LocationReportingRequestType

	messageMeta
}

var locationReportIEs = []ieSpec[LocationReport]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idUserLocationInformation, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			var v UserLocationInformation

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UserLocationInformation = &v

			return nil
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		id: idUEPresenceInAreaOfInterestList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UEPresenceInAreaOfInterestList)
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.UEPresenceInAreaOfInterestList == nil {
				return nil, false
			}

			return m.UEPresenceInAreaOfInterestList, true
		},
	},
	{
		id: idLocationReportingRequestType, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *LocationReport, raw []byte, enc per.Encoding) error {
			var v LocationReportingRequestType

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.LocationReportingRequestType = &v

			return nil
		},
		encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.LocationReportingRequestType == nil {
				return nil, false
			}

			return m.LocationReportingRequestType, true
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

// TS 38.413 §9.2.8.1. The AMF asks the NG-RAN node to start, change or stop
// reporting the UE's location. TS 36.413 §9.1.8.1 defines the same procedure
// with the same three IEs, but this core only drives it from the 5G side.
type LocationReportingControl struct {
	AMFUENGAPID                  AMFUENGAPID
	RANUENGAPID                  RANUENGAPID
	LocationReportingRequestType *LocationReportingRequestType

	messageMeta
}

var locationReportingControlIEs = []ieSpec[LocationReportingControl]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *LocationReportingControl, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *LocationReportingControl) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *LocationReportingControl, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *LocationReportingControl) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idLocationReportingRequestType, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *LocationReportingControl, raw []byte, enc per.Encoding) error {
			var v LocationReportingRequestType

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.LocationReportingRequestType = &v

			return nil
		},
		encode: func(m *LocationReportingControl) (per.Marshaler, bool) {
			if m.LocationReportingRequestType == nil {
				return nil, false
			}

			return m.LocationReportingRequestType, true
		},
	},
}

func (m *LocationReportingControl) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcLocationReportingControl, locationReportingControlIEs, m)
}

func (m *LocationReportingControl) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcLocationReportingControl,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseLocationReportingControl(value []byte) (*LocationReportingControl, error) {
	return parseMessageBody[LocationReportingControl](ProcLocationReportingControl, TriggeringInitiatingMessage, locationReportingControlIEs, value)
}
