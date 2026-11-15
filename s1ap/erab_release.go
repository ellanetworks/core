// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// ERABReleaseItemBearerRelComp ::= SEQUENCE { e-RAB-ID, iE-Extensions OPTIONAL }
// (extensible): an E-RAB the eNB confirms released (TS 36.413).
type ERABReleaseItemBearerRelComp struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	ERABID       ERABID
	iEExtensions extensionMeta `per:",exttable"`
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
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABReleaseCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABReleaseCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDUEAggregateMaximumBitrate, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *ERABReleaseCommand, r *per.Reader, enc per.Encoding) error {
			var (
				err  error
				ambr UEAggregateMaximumBitRate
			)

			err = decodeIE(r, &ambr)
			if err != nil {
				return err
			}

			m.UEAggregateMaximumBitRate = &ambr

			return nil
		},
		Encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		ID: IDERABToBeReleasedList, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseCommand, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABToBeReleased, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) {
			if len(m.ERABToBeReleased) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABToBeReleased), true
		},
	},
	{
		ID: IDNASPDU, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASPDU)
		},
		Encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) {
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

func DecodeERABReleaseCommand(value []byte) (*ERABReleaseCommand, error) {
	return decodeMessageBody[ERABReleaseCommand](ProcERABRelease, eRABReleaseCommandIEs, value)
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
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDERABReleaseListBearerRelComp, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABReleased, err = decodeItemList[ERABReleaseItemBearerRelComp](r, enc, maxnoofERABs, IDERABReleaseItemBearerRelComp, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if len(m.ERABReleased) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABReleaseItemBearerRelComp, CriticalityIgnore, m.ERABReleased), true
		},
	},
	{
		ID: IDERABFailedToReleaseList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABFailedToRelease, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if len(m.ERABFailedToRelease) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABFailedToRelease), true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var cd CriticalityDiagnostics

			if err := decodeIE(r, &cd); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &cd

			return nil
		},
		Encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseResponse, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
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

func DecodeERABReleaseResponse(value []byte) (*ERABReleaseResponse, error) {
	return decodeMessageBody[ERABReleaseResponse](ProcERABRelease, eRABReleaseResponseIEs, value)
}

type ERABReleaseIndication struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	ERABReleased            []ERABItem
	UserLocationInformation *UserLocationInformation

	messageMeta
}

var eRABReleaseIndicationIEs = []ieSpec[ERABReleaseIndication]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABReleaseIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *ERABReleaseIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ERABReleaseIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *ERABReleaseIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDERABReleasedList, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseIndication, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABReleased, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *ERABReleaseIndication) (per.Marshaler, bool) {
			if len(m.ERABReleased) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, m.ERABReleased), true
		},
	},
	{
		ID: IDUserLocationInformation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ERABReleaseIndication, r *per.Reader, enc per.Encoding) error {
			var uli UserLocationInformation

			if err := decodeIE(r, &uli); err != nil {
				return err
			}

			m.UserLocationInformation = &uli

			return nil
		},
		Encode: func(m *ERABReleaseIndication) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *ERABReleaseIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcERABReleaseIndication, eRABReleaseIndicationIEs, m)
}

func (m *ERABReleaseIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcERABReleaseIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseERABReleaseIndication(value []byte) (*ERABReleaseIndication, error) {
	return parseMessageBody[ERABReleaseIndication](ProcERABReleaseIndication, TriggeringInitiatingMessage, eRABReleaseIndicationIEs, value)
}

func DecodeERABReleaseIndication(value []byte) (*ERABReleaseIndication, error) {
	return decodeMessageBody[ERABReleaseIndication](ProcERABReleaseIndication, eRABReleaseIndicationIEs, value)
}
