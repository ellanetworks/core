// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// HandoverRequired is the HANDOVER REQUIRED message (TS 36.413), sent by
// the source eNB to start the Handover Preparation procedure. SourceToTarget is
// the opaque source-eNB-to-target-eNB transparent container relayed to the target.
type HandoverRequired struct {
	MMEUES1APID    MMEUES1APID
	ENBUES1APID    ENBUES1APID
	HandoverType   HandoverType
	Cause          Cause
	TargetID       TargetID
	SourceToTarget TransparentContainer

	unmodeledIEs
}

// handoverRequiredIEs is the HandoverRequired IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseHandoverRequired decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverRequired(value []byte) (*HandoverRequired, error) {
	return parseMessageBody[HandoverRequired](ProcHandoverPreparation, handoverRequiredIEs, value)
}

// HandoverCommand is the HANDOVER COMMAND message (TS 36.413), the
// successful outcome the MME returns to the source eNB. ERABToRelease lists the
// bearers the target did not admit (TS 23.401); TargetToSource
// is the opaque target-to-source transparent container.
type HandoverCommand struct {
	MMEUES1APID    MMEUES1APID
	ENBUES1APID    ENBUES1APID
	HandoverType   HandoverType
	ERABToRelease  []ERABItem
	TargetToSource TransparentContainer

	unmodeledIEs
}

// handoverCommandIEs is the HandoverCommand IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseHandoverCommand decodes the message from a successfulOutcome open-type
// payload.
func ParseHandoverCommand(value []byte) (*HandoverCommand, error) {
	return parseMessageBody[HandoverCommand](ProcHandoverPreparation, handoverCommandIEs, value)
}

// HandoverPreparationFailure is the HANDOVER PREPARATION FAILURE message
// (TS 36.413), the unsuccessful outcome the MME returns to the source
// eNB when the handover cannot be prepared. The UE keeps its source-eNB context.
type HandoverPreparationFailure struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Cause       Cause

	unmodeledIEs
}

// handoverPreparationFailureIEs is the HandoverPreparationFailure IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseHandoverPreparationFailure decodes the message from an unsuccessfulOutcome
// open-type payload.
func ParseHandoverPreparationFailure(value []byte) (*HandoverPreparationFailure, error) {
	return parseMessageBody[HandoverPreparationFailure](ProcHandoverPreparation, handoverPreparationFailureIEs, value)
}
