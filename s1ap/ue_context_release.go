// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// UES1APIDs is the UE-S1AP-IDs CHOICE (TS 36.413): either the
// UE-S1AP-ID-pair (both identities, the form an MME sends) or a bare
// MME-UE-S1AP-ID. Pair selects which alternative.
type UES1APIDs struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Pair        bool
}

func (u UES1APIDs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	if !u.Pair {
		if err := per.EncodeConstrainedWholeNumber(w, enc, 0, ues1apIDsChoiceRootCount-1, 1); err != nil {
			return err
		}

		return u.MMEUES1APID.MarshalPER(w, enc)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, ues1apIDsChoiceRootCount-1, 0); err != nil {
		return err
	}

	// UE-S1AP-ID-pair ::= SEQUENCE { mME-UE-S1AP-ID, eNB-UE-S1AP-ID,
	// iE-Extensions OPTIONAL } (extensible).
	w.WriteBit(false)
	w.WriteBit(false)

	if err := u.MMEUES1APID.MarshalPER(w, enc); err != nil {
		return err
	}

	return u.ENBUES1APID.MarshalPER(w, enc)
}

func (u *UES1APIDs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return err
	}

	if isExt {
		return fmt.Errorf("s1ap: UE-S1AP-IDs extension alternative unsupported")
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, ues1apIDsChoiceRootCount-1)
	if err != nil {
		return err
	}

	switch idx {
	case 0:
		extBit, err := r.ReadBit()
		if err != nil {
			return err
		}

		extContainer, err := r.ReadBit()
		if err != nil {
			return err
		}

		var mme MMEUES1APID
		if err := mme.UnmarshalPER(r, enc); err != nil {
			return err
		}

		var enb ENBUES1APID
		if err := enb.UnmarshalPER(r, enc); err != nil {
			return err
		}

		if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
			return err
		}

		*u = UES1APIDs{MMEUES1APID: mme, ENBUES1APID: enb, Pair: true}

		return nil
	default:
		var mme MMEUES1APID
		if err := mme.UnmarshalPER(r, enc); err != nil {
			return err
		}

		*u = UES1APIDs{MMEUES1APID: mme}

		return nil
	}
}

const ues1apIDsChoiceRootCount = 2

// TS 36.413 §9.1.4.6.
type UEContextReleaseCommand struct {
	UES1APIDs UES1APIDs
	Cause     *Cause
	messageMeta
}

var uEContextReleaseCommandIEs = []ieSpec[UEContextReleaseCommand]{
	{
		id: idUES1APIDs, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UEContextReleaseCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UES1APIDs)
		},
		encode: func(m *UEContextReleaseCommand) (per.Marshaler, bool) { return &m.UES1APIDs, true },
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseCommand, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *UEContextReleaseCommand) (per.Marshaler, bool) {
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

// TS 36.413 §9.1.4.7.
type UEContextReleaseComplete struct {
	MMEUES1APID             *MMEUES1APID
	ENBUES1APID             *ENBUES1APID
	CriticalityDiagnostics  *CriticalityDiagnostics
	UserLocationInformation *UserLocationInformation

	messageMeta
}

var uEContextReleaseCompleteIEs = []ieSpec[UEContextReleaseComplete]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			var (
				err error
				cd  CriticalityDiagnostics
			)

			err = perIEDecode(raw, &cd)
			m.CriticalityDiagnostics = &cd

			return err
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseComplete, raw []byte, enc per.Encoding) error {
			var (
				err error
				uli UserLocationInformation
			)

			err = perIEDecode(raw, &uli)
			m.UserLocationInformation = &uli

			return err
		},
		encode: func(m *UEContextReleaseComplete) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
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

// TS 36.413 §9.1.4.5.
type UEContextReleaseRequest struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       *Cause

	messageMeta
}

var uEContextReleaseRequestIEs = []ieSpec[UEContextReleaseRequest]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UEContextReleaseRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UEContextReleaseRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UEContextReleaseRequest, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *UEContextReleaseRequest) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
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
