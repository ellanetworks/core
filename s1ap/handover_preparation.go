// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 36.413 §9.1.5.1.
type HandoverRequired struct {
	MMEUES1APID  MMEUES1APID
	ENBUES1APID  ENBUES1APID
	HandoverType HandoverType
	Cause        *Cause
	TargetID     TargetID

	// DirectForwardingPathAvailability reports that a direct forwarding path to
	// the target is available. Absent means it is not, which is why it is a
	// pointer rather than a bool: TS 36.413 §9.2.3.15 gives the enumeration one
	// root value and carries "not available" as the IE's absence.
	DirectForwardingPathAvailability *DirectForwardingPathAvailability

	SourceToTarget                              TransparentContainer
	SRVCCHOIndication                           *SRVCCHOIndication
	SourceToTargetTransparentContainerSecondary *TransparentContainer
	MSClassmark2                                *MSClassmark2
	MSClassmark3                                *MSClassmark3
	CSGId                                       *CSGId
	CellAccessMode                              *CellAccessMode
	PSServiceNotAvailable                       *PSServiceNotAvailable

	messageMeta
}

var handoverRequiredIEs = []ieSpec[HandoverRequired]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequired, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequired, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDHandoverType, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequired, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.HandoverType)
		},
		Encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequired, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDTargetID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequired, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.TargetID)
		},
		Encode: func(m *HandoverRequired) (per.Marshaler, bool) { return &m.TargetID, true },
	},
	{
		ID: IDDirectForwardingPathAvailability, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequired, r *per.Reader, enc per.Encoding) error {
			var v DirectForwardingPathAvailability

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.DirectForwardingPathAvailability = &v

			return nil
		},
		Encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.DirectForwardingPathAvailability == nil {
				return nil, false
			}

			return m.DirectForwardingPathAvailability, true
		},
	},
	apimpl.OptionalIE(IDSRVCCHOIndication, CriticalityReject, func(m *HandoverRequired) **SRVCCHOIndication { return &m.SRVCCHOIndication }),
	{
		ID: IDSourceToTargetTransparentContainer, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequired, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SourceToTarget)
		},
		Encode: func(m *HandoverRequired) (per.Marshaler, bool) {
			if m.SourceToTarget == nil {
				return nil, false
			}

			return &m.SourceToTarget, true
		},
	},
	apimpl.OptionalIE(IDSourceToTargetTransparentContainerSecondary, CriticalityReject, func(m *HandoverRequired) **TransparentContainer {
		return &m.SourceToTargetTransparentContainerSecondary
	}),
	apimpl.ConditionalIE(IDMSClassmark2, CriticalityReject, handoverRequiredSRVCCToGERAN, func(m *HandoverRequired) **MSClassmark2 { return &m.MSClassmark2 }),
	apimpl.ConditionalIE(IDMSClassmark3, CriticalityIgnore, handoverRequiredSRVCCToGERAN, func(m *HandoverRequired) **MSClassmark3 { return &m.MSClassmark3 }),
	apimpl.OptionalIE(IDCSGId, CriticalityReject, func(m *HandoverRequired) **CSGId { return &m.CSGId }),
	apimpl.OptionalIE(IDCellAccessMode, CriticalityReject, func(m *HandoverRequired) **CellAccessMode { return &m.CellAccessMode }),
	apimpl.OptionalIE(IDPSServiceNotAvailable, CriticalityIgnore, func(m *HandoverRequired) **PSServiceNotAvailable { return &m.PSServiceNotAvailable }),
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

func DecodeHandoverRequired(value []byte) (*HandoverRequired, error) {
	return decodeMessageBody[HandoverRequired](ProcHandoverPreparation, handoverRequiredIEs, value)
}

// TS 36.413 §9.1.5.2. The MME tells the source eNB that the target has
// resources ready. TS 38.413 §9.2.3.2 carries the same shape; S1AP alone has a
// second target container for SRVCC.
type HandoverCommand struct {
	MMEUES1APID                                 MMEUES1APID
	ENBUES1APID                                 ENBUES1APID
	HandoverType                                HandoverType
	NASSecurityParametersfromEUTRAN             NASSecurityParametersfromEUTRAN
	ERABSubjecttoDataForwarding                 ERABSubjecttoDataForwardingList
	ERABToRelease                               []ERABItem
	TargetToSource                              TransparentContainer
	CriticalityDiagnostics                      *CriticalityDiagnostics
	TargetToSourceTransparentContainerSecondary *TransparentContainer

	messageMeta
}

// The NAS security parameters travel only when the UE leaves E-UTRAN: TS 36.413
// §9.1.5.2 condition iftoUTRANGERAN.
func handoverLeavesEUTRAN(m *HandoverCommand) bool {
	return m.HandoverType == HandoverTypeLTEtoUTRAN || m.HandoverType == HandoverTypeLTEtoGERAN
}

var handoverCommandIEs = []ieSpec[HandoverCommand]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDHandoverType, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.HandoverType)
		},
		Encode: func(m *HandoverCommand) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		ID: IDNASSecurityParametersfromEUTRAN, Presence: presenceConditional, Criticality: CriticalityReject,
		Condition: handoverLeavesEUTRAN,
		Decode: func(m *HandoverCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASSecurityParametersfromEUTRAN)
		},
		Encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.NASSecurityParametersfromEUTRAN == nil {
				return nil, false
			}

			return m.NASSecurityParametersfromEUTRAN, true
		},
	},
	{
		ID: IDERABSubjecttoDataForwardingList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverCommand, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABSubjecttoDataForwarding, err = decodeItemList[ERABDataForwardingItem](r, enc, maxnoofERABs, IDERABDataForwardingItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if len(m.ERABSubjecttoDataForwarding) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABDataForwardingItem, CriticalityIgnore, &m.ERABSubjecttoDataForwarding), true
		},
	},
	{
		ID: IDERABtoReleaseListHOCmd, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverCommand, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABToRelease, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if len(m.ERABToRelease) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABItem, CriticalityIgnore, &m.ERABToRelease), true
		},
	},
	{
		ID: IDTargetToSourceTransparentContainer, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverCommand, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.TargetToSource)
		},
		Encode: func(m *HandoverCommand) (per.Marshaler, bool) {
			if m.TargetToSource == nil {
				return nil, false
			}

			return &m.TargetToSource, true
		},
	},
	apimpl.OptionalIE(IDTargetToSourceTransparentContainerSecondary, CriticalityReject, func(m *HandoverCommand) **TransparentContainer { return &m.TargetToSourceTransparentContainerSecondary }),
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverCommand, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *HandoverCommand) (per.Marshaler, bool) {
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

func DecodeHandoverCommand(value []byte) (*HandoverCommand, error) {
	return decodeMessageBody[HandoverCommand](ProcHandoverPreparation, handoverCommandIEs, value)
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
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverPreparationFailure, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverPreparationFailure, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverPreparationFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverPreparationFailure, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *HandoverPreparationFailure) (per.Marshaler, bool) {
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

func DecodeHandoverPreparationFailure(value []byte) (*HandoverPreparationFailure, error) {
	return decodeMessageBody[HandoverPreparationFailure](ProcHandoverPreparation, handoverPreparationFailureIEs, value)
}

func handoverRequiredSRVCCToGERAN(m *HandoverRequired) bool {
	return m.HandoverType == HandoverTypeLTEtoGERAN && m.SRVCCHOIndication != nil
}
