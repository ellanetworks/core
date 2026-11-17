// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// maxnoofAoI bounds the areas of interest one request may name.
const maxnoofAoI = 64

// EventType ::= ENUMERATED { direct, change-of-serve-cell,
// ue-presence-in-area-of-interest, stop-change-of-serve-cell,
// stop-ue-presence-in-area-of-interest, cancel-location-reporting-for-the-ue,
// ... }, an inline type of the Location Reporting Request Type IE
// (TS 38.413 §9.3.1.65).
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
	EventTypeChangeOfServingCellAndUEPresenceInAreaOfInterest
	EventTypeReportAerialUEFlightInformation
	EventTypeCancelAerialUEFlightInformationReporting
)

// ReportArea ::= ENUMERATED { cell, ... }, likewise inline in TS 38.413
// §9.3.1.65. TS 36.413
// names its single member ecgi; the wire encoding is the same.
type ReportArea uint8

const (
	ReportAreaCell ReportArea = iota
)

// LocationReportingReferenceID ::= INTEGER (1..64, ...) — TS 38.413 §9.3.1.76.
type LocationReportingReferenceID uint8

func (id LocationReportingReferenceID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, maxnoofAoI, int64(id))
}

func (id *LocationReportingReferenceID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[LocationReportingReferenceID](r, enc, 1, maxnoofAoI)
	if err != nil {
		return err
	}

	*id = v

	return nil
}

func (v LocationReportingReferenceID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, maxnoofAoI)
}

// LocationReportingRequestType ::= SEQUENCE { eventType, reportArea,
// areaOfInterestList OPTIONAL, locationReportingReferenceIDToBeCancelled
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.65.
// TS 36.413 §9.2.1.34 names the same shape RequestType and has neither optional
// field.
type LocationReportingRequestType struct {
	EventType                                           EventType
	ReportArea                                          ReportArea
	AreaOfInterestList                                  AreaOfInterestList                                  `per:",optional"`
	LocationReportingReferenceIDToBeCancelled           *LocationReportingReferenceID                       `per:",optional"`
	LocationReportingAdditionalInfo                     *LocationReportingAdditionalInfo                    `per:",extie:IDLocationReportingAdditionalInfo,crit:ignore"`
	AdditionalCancelledlocationReportingReferenceIDList AdditionalCancelledlocationReportingReferenceIDList `per:",extie:IDAdditionalCancelledlocationReportingReferenceIDList,crit:reject"`
	AerialUEFlightInformationReportingControlList       AerialUEFlightInformationReportingControlList       `per:",extie:IDAerialUEFlightInformationReportingControlList,crit:ignore"`
	iEExtensions                                        extensionMeta                                       `per:",exttable"`
	extensions                                          per.ExtensionAdditions                              `per:",extraw"`
}

const (
	maxnoofAoIMinusOne             = 63
	maxnoofTAIinAoI                = 16
	maxnoofCellinAoI               = 256
	maxnoofRANNodeinAoI            = 64
	maxnoofFlightInfoReportControl = 64
)

type AreaOfInterestItem struct {
	AreaOfInterest               AreaOfInterest
	LocationReportingReferenceID LocationReportingReferenceID
	iEExtensions                 extensionMeta          `per:",exttable"`
	extensions                   per.ExtensionAdditions `per:",extraw"`
}

type AreaOfInterestList []AreaOfInterestItem

func (l AreaOfInterestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAoI, []AreaOfInterestItem(l))
}

func (l *AreaOfInterestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AreaOfInterestItem](r, enc, 1, maxnoofAoI)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AreaOfInterest struct {
	AreaOfInterestTAIList     AreaOfInterestTAIList     `per:",optional"`
	AreaOfInterestCellList    AreaOfInterestCellList    `per:",optional"`
	AreaOfInterestRANNodeList AreaOfInterestRANNodeList `per:",optional"`
	iEExtensions              extensionMeta             `per:",exttable"`
	extensions                per.ExtensionAdditions    `per:",extraw"`
}

type AreaOfInterestTAIItem struct {
	TAI          TAI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type AreaOfInterestTAIList []AreaOfInterestTAIItem

func (l AreaOfInterestTAIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIinAoI, []AreaOfInterestTAIItem(l))
}

func (l *AreaOfInterestTAIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AreaOfInterestTAIItem](r, enc, 1, maxnoofTAIinAoI)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AreaOfInterestCellItem struct {
	NGRANCGI     NGRANCGI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type AreaOfInterestCellList []AreaOfInterestCellItem

func (l AreaOfInterestCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellinAoI, []AreaOfInterestCellItem(l))
}

func (l *AreaOfInterestCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AreaOfInterestCellItem](r, enc, 1, maxnoofCellinAoI)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AreaOfInterestRANNodeItem struct {
	GlobalRANNodeID GlobalRANNodeID
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type AreaOfInterestRANNodeList []AreaOfInterestRANNodeItem

func (l AreaOfInterestRANNodeList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofRANNodeinAoI, []AreaOfInterestRANNodeItem(l))
}

func (l *AreaOfInterestRANNodeList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AreaOfInterestRANNodeItem](r, enc, 1, maxnoofRANNodeinAoI)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type LocationReportingAdditionalInfo uint8

const (
	LocationReportingAdditionalInfoIncludePSCell LocationReportingAdditionalInfo = iota
)

type AdditionalCancelledlocationReportingReferenceIDItem struct {
	LocationReportingReferenceIDToBeCancelled LocationReportingReferenceID
	iEExtensions                              extensionMeta          `per:",exttable"`
	extensions                                per.ExtensionAdditions `per:",extraw"`
}

type AdditionalCancelledlocationReportingReferenceIDList []AdditionalCancelledlocationReportingReferenceIDItem

func (l AdditionalCancelledlocationReportingReferenceIDList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAoIMinusOne, []AdditionalCancelledlocationReportingReferenceIDItem(l))
}

func (l *AdditionalCancelledlocationReportingReferenceIDList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AdditionalCancelledlocationReportingReferenceIDItem](r, enc, 1, maxnoofAoIMinusOne)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AerialUEFlightInformationReportingControlItem struct {
	AerialUEFlightInformationReportingControl AerialUEFlightInformationReportingControl
	iEExtensions                              extensionMeta          `per:",exttable"`
	extensions                                per.ExtensionAdditions `per:",extraw"`
}

type AerialUEFlightInformationReportingControlList []AerialUEFlightInformationReportingControlItem

func (l AerialUEFlightInformationReportingControlList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofFlightInfoReportControl, []AerialUEFlightInformationReportingControlItem(l))
}

func (l *AerialUEFlightInformationReportingControlList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AerialUEFlightInformationReportingControlItem](r, enc, 1, maxnoofFlightInfoReportControl)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AerialUEFlightInformationReportingControl struct {
	HigherAltitudeThreshold   Altitude                      `per:",range:-420..10000,..."`
	LowerAltitudeThreshold    Altitude                      `per:",range:-420..10000,..."`
	AerialUEReportPeriodicity *AerialUEReportingPeriodicity `per:",optional"`
	AreaID                    AreaID
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type AerialUEReportingPeriodicity uint8

const (
	AerialUEReportingPeriodicityMs120 AerialUEReportingPeriodicity = iota
	AerialUEReportingPeriodicityMs240
	AerialUEReportingPeriodicityMs480
	AerialUEReportingPeriodicityMs640
	AerialUEReportingPeriodicityMs1024
	AerialUEReportingPeriodicityMs2048
	AerialUEReportingPeriodicityMs5120
	AerialUEReportingPeriodicityMs10240
	AerialUEReportingPeriodicityMs20480
	AerialUEReportingPeriodicityMs40960
	AerialUEReportingPeriodicityMin1
	AerialUEReportingPeriodicityMin6
	AerialUEReportingPeriodicityMin12
	AerialUEReportingPeriodicityMin30
)

type AreaID struct {
	TAI             *TAI             `per:",choice:0"`
	GlobalRANNodeID *GlobalRANNodeID `per:",choice:1"`
	NRCGI           *NRCGI           `per:",choice:2"`
	ChoiceExtension *SingleContainer `per:",choice:3,extie"`
}

// UEPresence ::= ENUMERATED { in, out, unknown, ... }, an inline type of the
// UE Presence in Area of Interest List IE (TS 38.413 §9.3.1.67).
type UEPresence uint8

const (
	UEPresenceIn UEPresence = iota
	UEPresenceOut
	UEPresenceUnknown
)

// UEPresenceInAreaOfInterestItem ::= SEQUENCE { locationReportingReferenceID,
// uEPresence, iE-Extensions OPTIONAL } (extensible).
type UEPresenceInAreaOfInterestItem struct {
	extensions                   per.ExtensionAdditions `per:",extraw"`
	LocationReportingReferenceID LocationReportingReferenceID
	UEPresence                   UEPresence
	iEExtensions                 extensionMeta `per:",exttable"`
}

// UEPresenceInAreaOfInterestList ::= SEQUENCE (SIZE(1..maxnoofAoI)) OF
// UEPresenceInAreaOfInterestItem.
type UEPresenceInAreaOfInterestList []UEPresenceInAreaOfInterestItem

// TS 38.413 §9.2.11.3. The NG-RAN node reports where the UE is. TS 36.413
// §9.1.12.3 splits the location into EUTRAN-CGI and TAI where NGAP carries one
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
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDUserLocationInformation, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			var v UserLocationInformation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.UserLocationInformation = &v

			return nil
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	{
		ID: IDUEPresenceInAreaOfInterestList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UEPresenceInAreaOfInterestList)
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) {
			if m.UEPresenceInAreaOfInterestList == nil {
				return nil, false
			}

			return m.UEPresenceInAreaOfInterestList, true
		},
	},
	{
		ID: IDLocationReportingRequestType, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *LocationReport, r *per.Reader, enc per.Encoding) error {
			var v LocationReportingRequestType

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.LocationReportingRequestType = &v

			return nil
		},
		Encode: func(m *LocationReport) (per.Marshaler, bool) {
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

func DecodeLocationReport(value []byte) (*LocationReport, error) {
	return decodeMessageBody[LocationReport](ProcLocationReport, locationReportIEs, value)
}

// TS 38.413 §9.2.11.1. The AMF asks the NG-RAN node to start, change or stop
// reporting the UE's location. TS 36.413 §9.1.12.1 defines the same procedure
// with the same three IEs, but this core only drives it from the 5G side.
type LocationReportingControl struct {
	AMFUENGAPID                  AMFUENGAPID
	RANUENGAPID                  RANUENGAPID
	LocationReportingRequestType *LocationReportingRequestType

	messageMeta
}

var locationReportingControlIEs = []ieSpec[LocationReportingControl]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *LocationReportingControl, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *LocationReportingControl) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *LocationReportingControl, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *LocationReportingControl) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDLocationReportingRequestType, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *LocationReportingControl, r *per.Reader, enc per.Encoding) error {
			var v LocationReportingRequestType

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.LocationReportingRequestType = &v

			return nil
		},
		Encode: func(m *LocationReportingControl) (per.Marshaler, bool) {
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

func DecodeLocationReportingControl(value []byte) (*LocationReportingControl, error) {
	return decodeMessageBody[LocationReportingControl](ProcLocationReportingControl, locationReportingControlIEs, value)
}
