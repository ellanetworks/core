// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABReleaseItemBearerRelComp ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible): an E-RAB the eNB confirms released (TS 36.413).
type ERABReleaseItemBearerRelComp struct {
	_      [0]struct{} `per:"extseq"`
	ERABID ERABID
	_      ieExtensions `per:",skip"`
}

// TS 36.413 §9.1.3.5.
type ERABReleaseCommand struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeReleased          []ERABItem
	NASPDU                    NASPDU

	messageMeta
}

var eRABReleaseCommandIEs = []ieSpec[ERABReleaseCommand]{
	{
		id: IDMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: IDUEAggregateMaximumBitrate, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			var (
				err  error
				ambr UEAggregateMaximumBitRate
			)

			err = perIEDecode(raw, &ambr)
			if err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &ambr

			return nil
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		id: IDERABToBeReleasedList, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeReleased, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) {
			if len(m.ERABToBeReleased) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABToBeReleased)
			}), true
		},
	},
	{
		id: IDNASPDU, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASPDU)
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) {
			if len(m.NASPDU) == 0 {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
}

func (m *ERABReleaseCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABRelease, eRABReleaseCommandIEs, m)
}

func (m *ERABReleaseCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseERABReleaseCommand(value []byte) (*ERABReleaseCommand, error) {
	return parseMessageBody[ERABReleaseCommand](ProcERABRelease, TriggeringInitiatingMessage, eRABReleaseCommandIEs, value)
}

// TS 36.413 §9.1.3.6.
type ERABReleaseResponse struct {
	MMEUES1APID             *MMEUES1APID
	ENBUES1APID             *ENBUES1APID
	ERABReleased            []ERABReleaseItemBearerRelComp
	ERABFailedToRelease     []ERABItem
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	messageMeta
}

var eRABReleaseResponseIEs = []ieSpec[ERABReleaseResponse]{
	{
		id: IDMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: IDENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: IDERABReleaseListBearerRelComp, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABReleased, err = decodeItemList[ERABReleaseItemBearerRelComp](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if len(m.ERABReleased) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABReleaseItemBearerRelComp, CriticalityIgnore, m.ERABReleased)
			}), true
		},
	},
	{
		id: IDERABFailedToReleaseList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABFailedToRelease, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToRelease) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABFailedToRelease)
			}), true
		},
	},
	{
		id: IDCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := perIEDecode(raw, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: IDUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := perIEDecode(raw, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *ERABReleaseResponse) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABRelease, eRABReleaseResponseIEs, m)
}

func (m *ERABReleaseResponse) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcERABRelease,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseERABReleaseResponse(value []byte) (*ERABReleaseResponse, error) {
	return parseMessageBody[ERABReleaseResponse](ProcERABRelease, TriggeringSuccessfulOutcome, eRABReleaseResponseIEs, value)
}
