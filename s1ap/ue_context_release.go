// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// UE-S1AP-IDs ::= CHOICE { uE-S1AP-ID-pair, mME-UE-S1AP-ID, ... }. The MME
// sends uE-S1AP-ID-pair once the eNB has assigned an eNB UE S1AP ID; otherwise
// only mME-UE-S1AP-ID is carried.
type UES1APIDs struct {
	UES1APIDPair *UES1APIDPair             `per:",choice:0"`
	MMEUES1APID  *MMEUES1APID              `per:",choice:1"`
	Ext          *per.ExtensionAlternative `per:",extalt"`
}

// UE-S1AP-ID-pair ::= SEQUENCE { mME-UE-S1AP-ID, eNB-UE-S1AP-ID,
// iE-Extensions OPTIONAL } (extensible).
type UES1APIDPair struct {
	MMEUES1APID  MMEUES1APID
	ENBUES1APID  ENBUES1APID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

func (u UES1APIDs) MMEID() (MMEUES1APID, bool) {
	switch {
	case u.UES1APIDPair != nil:
		return u.UES1APIDPair.MMEUES1APID, true
	case u.MMEUES1APID != nil:
		return *u.MMEUES1APID, true
	default:
		return 0, false
	}
}

func (u UES1APIDs) ENBID() (ENBUES1APID, bool) {
	if u.UES1APIDPair == nil {
		return 0, false
	}

	return u.UES1APIDPair.ENBUES1APID, true
}

// TS 36.413 §9.1.4.6.
type UEContextReleaseCommand struct {
	UES1APIDs UES1APIDs
	Cause     *Cause

	messageMeta
}

var uEContextReleaseCommandIEs = []ieSpec[UEContextReleaseCommand]{
	{
		ID: IDUES1APIDs, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UES1APIDs)
		},
		Encode: func(m *UEContextReleaseCommand) (per.Marshaler, bool) { return &m.UES1APIDs, true },
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseCommand, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *UEContextReleaseCommand) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
}

func (m *UEContextReleaseCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextRelease, uEContextReleaseCommandIEs, m)
}

func (m *UEContextReleaseCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextReleaseCommand(value []byte) (*UEContextReleaseCommand, error) {
	return parseMessageBody[UEContextReleaseCommand](ProcUEContextRelease, TriggeringInitiatingMessage, uEContextReleaseCommandIEs, value)
}

func DecodeUEContextReleaseCommand(value []byte) (*UEContextReleaseCommand, error) {
	return decodeMessageBody[UEContextReleaseCommand](ProcUEContextRelease, uEContextReleaseCommandIEs, value)
}

// TS 36.413 §9.1.4.7.
type UEContextReleaseComplete struct {
	MMEUES1APID                                   *MMEUES1APID
	ENBUES1APID                                   *ENBUES1APID
	CriticalityDiagnostics                        *CriticalityDiagnostics
	UserLocationInformation                       *UserLocationInformation
	InformationOnRecommendedCellsAndENBsForPaging *InformationOnRecommendedCellsAndENBsForPaging
	CellIdentifierAndCELevelForCECapableUEs       *CellIdentifierAndCELevelForCECapableUEs
	SecondaryRATDataUsageReportList               SecondaryRATDataUsageReportList
	TimeSinceSecondaryNodeRelease                 *TimeSinceSecondaryNodeRelease

	messageMeta
}

var uEContextReleaseCompleteIEs = []ieSpec[UEContextReleaseComplete]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseComplete, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
	apimpl.OptionalIE(IDInformationOnRecommendedCellsAndENBsForPaging, CriticalityIgnore, func(m *UEContextReleaseComplete) **InformationOnRecommendedCellsAndENBsForPaging {
		return &m.InformationOnRecommendedCellsAndENBsForPaging
	}),
	apimpl.OptionalIE(IDCellIdentifierAndCELevelForCECapableUEs, CriticalityIgnore, func(m *UEContextReleaseComplete) **CellIdentifierAndCELevelForCECapableUEs {
		return &m.CellIdentifierAndCELevelForCECapableUEs
	}),
	apimpl.ListIE(IDSecondaryRATDataUsageReportList, CriticalityIgnore, func(m *UEContextReleaseComplete) *SecondaryRATDataUsageReportList {
		return &m.SecondaryRATDataUsageReportList
	}),
	apimpl.OptionalIE(IDTimeSinceSecondaryNodeRelease, CriticalityIgnore, func(m *UEContextReleaseComplete) **TimeSinceSecondaryNodeRelease {
		return &m.TimeSinceSecondaryNodeRelease
	}),
}

func (m *UEContextReleaseComplete) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextRelease, uEContextReleaseCompleteIEs, m)
}

func (m *UEContextReleaseComplete) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcUEContextRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUEContextReleaseComplete(value []byte) (*UEContextReleaseComplete, error) {
	return parseMessageBody[UEContextReleaseComplete](ProcUEContextRelease, TriggeringSuccessfulOutcome, uEContextReleaseCompleteIEs, value)
}

func DecodeUEContextReleaseComplete(value []byte) (*UEContextReleaseComplete, error) {
	return decodeMessageBody[UEContextReleaseComplete](ProcUEContextRelease, uEContextReleaseCompleteIEs, value)
}

// TS 36.413 §9.1.4.5.
type UEContextReleaseRequest struct {
	MMEUES1APID                     MMEUES1APID
	ENBUES1APID                     ENBUES1APID
	Cause                           *Cause
	GWContextReleaseIndication      *GWContextReleaseIndication
	SecondaryRATDataUsageReportList SecondaryRATDataUsageReportList

	messageMeta
}

var uEContextReleaseRequestIEs = []ieSpec[UEContextReleaseRequest]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDGWContextReleaseIndication, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *UEContextReleaseRequest, r *per.Reader, enc per.Encoding) error {
			var v GWContextReleaseIndication

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.GWContextReleaseIndication = &v

			return nil
		},
		Encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) {
			if m.GWContextReleaseIndication == nil {
				return nil, false
			}

			return m.GWContextReleaseIndication, true
		},
	},
	apimpl.ListIE(IDSecondaryRATDataUsageReportList, CriticalityIgnore, func(m *UEContextReleaseRequest) *SecondaryRATDataUsageReportList {
		return &m.SecondaryRATDataUsageReportList
	}),
}

func (m *UEContextReleaseRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUEContextReleaseRequest, uEContextReleaseRequestIEs, m)
}

func (m *UEContextReleaseRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUEContextReleaseRequest,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUEContextReleaseRequest(value []byte) (*UEContextReleaseRequest, error) {
	return parseMessageBody[UEContextReleaseRequest](ProcUEContextReleaseRequest, TriggeringInitiatingMessage, uEContextReleaseRequestIEs, value)
}

func DecodeUEContextReleaseRequest(value []byte) (*UEContextReleaseRequest, error) {
	return decodeMessageBody[UEContextReleaseRequest](ProcUEContextReleaseRequest, uEContextReleaseRequestIEs, value)
}
