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
	Cause          Cause
	TargetID       TargetID
	SourceToTarget TransparentContainer

	unmodeledIEs
}

var handoverRequiredIEs = []ieSpec[HandoverRequired]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idHandoverType, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.Cause, true },
	},
	{
		id: idTargetID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetID)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.TargetID, true },
	},
	{
		id: idSourceToTargetTransparentContainer, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequired, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceToTarget)
		},
		encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.SourceToTarget, true },
	},
}

func (m *HandoverRequired) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverRequiredIEs, m)
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
	return parseMessageBody[HandoverRequired](ProcHandoverPreparation, handoverRequiredIEs, value)
}

// TS 36.413 §9.1.5.2.
type HandoverCommand struct {
	MMEUES1APID    MMEUES1APID
	ENBUES1APID    ENBUES1APID
	HandoverType   HandoverType
	ERABToRelease  []ERABItem
	TargetToSource TransparentContainer

	unmodeledIEs
}

var handoverCommandIEs = []ieSpec[HandoverCommand]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idHandoverType, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: idERABtoReleaseListHOCmd, presence: PresenceOptional, crit: CriticalityIgnore,
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
		id: idTargetToSourceTransparentContainer, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverCommand, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetToSource)
		},
		encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.TargetToSource, true },
	},
}

func (m *HandoverCommand) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverCommandIEs, m)
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
	return parseMessageBody[HandoverCommand](ProcHandoverPreparation, handoverCommandIEs, value)
}

// TS 36.413 §9.1.5.3.
type HandoverPreparationFailure struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       Cause

	unmodeledIEs
}

var handoverPreparationFailureIEs = []ieSpec[HandoverPreparationFailure]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverPreparationFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) { return &m.Cause, true },
	},
}

func (m *HandoverPreparationFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverPreparationFailureIEs, m)
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
	return parseMessageBody[HandoverPreparationFailure](ProcHandoverPreparation, handoverPreparationFailureIEs, value)
}
