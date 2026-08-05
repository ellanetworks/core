// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.5.1.
type HandoverRequired struct {
	MMEUES1APID    MMEUES1APID
	ENBUES1APID    ENBUES1APID
	HandoverType   HandoverType
	Cause          *Cause
	TargetID       TargetID
	SourceToTarget TransparentContainer

	messageMeta
}

var handoverRequiredIEs = []ieSpec[HandoverRequired]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idHandoverType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idTargetID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.TargetID, true },
	},
	{
		id: idSourceToTargetTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceToTarget)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.SourceToTarget == nil {
				return nil, false
			}

			return &m.SourceToTarget, true
		},
	},
}

func (m *HandoverRequired) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverPreparation, handoverRequiredIEs, m)
}

func (m *HandoverRequired) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverRequired(value []byte) (*HandoverRequired, error) {
	return parseMessageBody[HandoverRequired](ProcHandoverPreparation, TriggeringInitiatingMessage, handoverRequiredIEs, value)
}

// TS 36.413 §9.1.5.2. The MME tells the source eNB that the target has
// resources ready. TS 38.413 §9.2.3.2 carries the same shape; S1AP alone has a
// second target container for SRVCC, which is not modelled.
type HandoverCommand struct {
	MMEUES1APID                     MMEUES1APID
	ENBUES1APID                     ENBUES1APID
	HandoverType                    HandoverType
	NASSecurityParametersfromEUTRAN NASSecurityParametersfromEUTRAN
	ERABSubjecttoDataForwarding     []ERABDataForwardingItem
	ERABToRelease                   []ERABItem
	TargetToSource                  TransparentContainer
	CriticalityDiagnostics          *CriticalityDiagnostics

	messageMeta
}

// The NAS security parameters travel only when the UE leaves E-UTRAN: TS 36.413
// §9.1.5.2 condition iftoUTRANGERAN.
func handoverLeavesEUTRAN(m *HandoverCommand) bool {
	return m.HandoverType == HandoverTypeLTEtoUTRAN || m.HandoverType == HandoverTypeLTEtoGERAN
}

var handoverCommandIEs = []ieSpec[HandoverCommand]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idHandoverType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: idNASSecurityParametersfromEUTRAN, presence: presenceConditional, crit: CriticalityReject,
		condition: handoverLeavesEUTRAN,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASSecurityParametersfromEUTRAN)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.NASSecurityParametersfromEUTRAN == nil {
				return nil, false
			}

			return m.NASSecurityParametersfromEUTRAN, true
		},
	},
	{
		id: idERABSubjecttoDataForwardingList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABSubjecttoDataForwarding, err = decodeItemList[ERABDataForwardingItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if len(m.ERABSubjecttoDataForwarding) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABDataForwardingItem, CriticalityIgnore, m.ERABSubjecttoDataForwarding)
			}), true
		},
	},
	{
		id: idERABtoReleaseListHOCmd, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToRelease, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if len(m.ERABToRelease) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABToRelease)
			}), true
		},
	},
	{
		id: idTargetToSourceTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetToSource)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.TargetToSource == nil {
				return nil, false
			}

			return &m.TargetToSource, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *HandoverCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverPreparation, handoverCommandIEs, m)
}

func (m *HandoverCommand) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverCommand(value []byte) (*HandoverCommand, error) {
	return parseMessageBody[HandoverCommand](ProcHandoverPreparation, TriggeringSuccessfulOutcome, handoverCommandIEs, value)
}

// TS 36.413 §9.1.5.3.
type HandoverPreparationFailure struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var handoverPreparationFailureIEs = []ieSpec[HandoverPreparationFailure]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idCriticalityDiagnostics, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *HandoverPreparationFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverPreparation, handoverPreparationFailureIEs, m)
}

func (m *HandoverPreparationFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcHandoverPreparation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverPreparationFailure(value []byte) (*HandoverPreparationFailure, error) {
	return parseMessageBody[HandoverPreparationFailure](ProcHandoverPreparation, TriggeringUnsuccessfulOutcome, handoverPreparationFailureIEs, value)
}
