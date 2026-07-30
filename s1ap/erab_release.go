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

// ERABReleaseCommand is the E-RAB RELEASE COMMAND message (TS 36.413),
// sent by the MME to release one or more E-RABs of a UE that stays connected —
// the radio leg of a PDN connection being disconnected (TS 23.401,
// "Deactivate Bearer Request"). The DEACTIVATE EPS BEARER CONTEXT REQUEST NAS
// message rides in the optional NAS-PDU IE, so the eNB releases the radio bearer
// and delivers the NAS in one step.
type ERABReleaseCommand struct {
	MMEUES1APID               MMEUES1APID
	ENBUES1APID               ENBUES1APID
	UEAggregateMaximumBitRate *UEAggregateMaximumBitRate
	ERABToBeReleased          []ERABItem
	NASPDU                    NASPDU

	unmodeledIEs
}

// eRABReleaseCommandIEs is the ERABReleaseCommand IE table (TS 36.413).
var eRABReleaseCommandIEs = []ieSpec[ERABReleaseCommand]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idUEAggregateMaximumBitrate, presence: PresenceOptional, crit: CriticalityReject,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			var (
				err  error
				ambr UEAggregateMaximumBitRate
			)

			err = perIEDecode(raw, &ambr)
			m.UEAggregateMaximumBitRate = &ambr

			return err
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) {
			if m.UEAggregateMaximumBitRate == nil {
				return nil, false
			}

			return m.UEAggregateMaximumBitRate, true
		},
	},
	{
		id: idERABToBeReleasedList, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseCommand, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeReleased, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *ERABReleaseCommand) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABToBeReleased)
			}), true
		},
	},
	{
		id: idNASPDU, presence: PresenceOptional, crit: CriticalityIgnore,
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
	return encodeMessageBody(w, enc, eRABReleaseCommandIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseERABReleaseCommand decodes the message from an initiatingMessage open-type
// payload.
func ParseERABReleaseCommand(value []byte) (*ERABReleaseCommand, error) {
	return parseMessageBody[ERABReleaseCommand](ProcERABRelease, eRABReleaseCommandIEs, value)
}

// ERABReleaseResponse is the E-RAB RELEASE RESPONSE message (TS 36.413),
// sent by the eNB once the E-RAB(s) are released.
type ERABReleaseResponse struct {
	MMEUES1APID             MMEUES1APID
	ENBUES1APID             ENBUES1APID
	ERABReleased            []ERABReleaseItemBearerRelComp
	ERABFailedToRelease     []ERABItem
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	unmodeledIEs
}

// eRABReleaseResponseIEs is the ERABReleaseResponse IE table (TS 36.413).
var eRABReleaseResponseIEs = []ieSpec[ERABReleaseResponse]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idERABReleaseListBearerRelComp, presence: PresenceOptional, crit: CriticalityIgnore,
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
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABReleaseItemBearerRelComp, CriticalityIgnore, m.ERABReleased)
			}), true
		},
	},
	{
		id: idERABFailedToReleaseList, presence: PresenceOptional, crit: CriticalityIgnore,
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
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToRelease)
			}), true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *ERABReleaseResponse) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: idUserLocationInformation, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ERABReleaseResponse, raw []byte, enc per.Encoding) error {
			var (
				err error
				uli UserLocationInformation
			)

			err = perIEDecode(raw, &uli)
			m.UserLocationInformation = &uli

			return err
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
	return encodeMessageBody(w, enc, eRABReleaseResponseIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseERABReleaseResponse decodes the message from a successfulOutcome open-type
// payload.
func ParseERABReleaseResponse(value []byte) (*ERABReleaseResponse, error) {
	return parseMessageBody[ERABReleaseResponse](ProcERABRelease, eRABReleaseResponseIEs, value)
}
