// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.5.4.
type HandoverRequest struct {
	MMEUES1APID            MMEUES1APID
	HandoverType           HandoverType
	Cause                  *Cause
	UEAMBR                 UEAggregateMaximumBitRate
	ERABToBeSetup          ERABToBeSetupListHOReq
	SourceToTarget         TransparentContainer
	UESecurityCapabilities UESecurityCapabilities

	// HandoverRestrictionList is optional in the ASN.1 and not optional in
	// practice: TS 36.413 §8.4.2.4 has the target eNB reject the handover when
	// the list is absent and it cannot determine the serving PLMN otherwise.
	HandoverRestrictionList *HandoverRestrictionList
	TraceActivation         *TraceActivation

	SecurityContext SecurityContext
	// C-iffromUTRANGERAN: present exactly when Handover Type is UTRANtoLTE or
	// GERANtoLTE (§9.1.5.4).
	NASSecurityParameterstoEUTRAN NASSecurityParameterstoEUTRAN
	GUMMEI                        *GUMMEI
	MMEUES1APID2                  *MMEUES1APID
	TimeRefDistribution           *TimeRefDistribution

	messageMeta
}

// The NAS security parameters travel only when the UE enters E-UTRAN: TS 36.413
// §9.1.5.4 condition iffromUTRANGERAN. It is the mirror of handoverLeavesEUTRAN.
func handoverEntersEUTRAN(m *HandoverRequest) bool {
	return m.HandoverType == HandoverTypeUTRANtoLTE || m.HandoverType == HandoverTypeGERANtoLTE
}

var handoverRequestIEs = []ieSpec[HandoverRequest]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDHandoverType, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.HandoverType)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDUEAggregateMaximumBitrate, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UEAMBR)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UEAMBR, true },
	},
	{
		ID: IDERABToBeSetupListHOReq, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemHOReq](r, enc, maxnoofERABs, IDERABToBeSetupItemHOReq, CriticalityReject)

			return err
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			return containerList(maxnoofERABs, IDERABToBeSetupItemHOReq, CriticalityReject, m.ERABToBeSetup), true
		},
	},
	{
		ID: IDSourceToTargetTransparentContainer, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SourceToTarget)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.SourceToTarget == nil {
				return nil, false
			}

			return &m.SourceToTarget, true
		},
	},
	{
		ID: IDUESecurityCapabilities, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.UESecurityCapabilities)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UESecurityCapabilities, true },
	},
	{
		ID: IDHandoverRestrictionList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v HandoverRestrictionList

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.HandoverRestrictionList = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.HandoverRestrictionList == nil {
				return nil, false
			}

			return m.HandoverRestrictionList, true
		},
	},
	{
		ID: IDTraceActivation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v TraceActivation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TraceActivation = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.TraceActivation == nil {
				return nil, false
			}

			return m.TraceActivation, true
		},
	},
	{
		ID: IDSecurityContext, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.SecurityContext)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
	{
		ID: IDNASSecurityParameterstoEUTRAN, Presence: presenceConditional, Criticality: CriticalityReject,
		Condition: handoverEntersEUTRAN,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASSecurityParameterstoEUTRAN)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.NASSecurityParameterstoEUTRAN == nil {
				return nil, false
			}

			return m.NASSecurityParameterstoEUTRAN, true
		},
	},
	{
		ID: IDGUMMEI, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v GUMMEI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.GUMMEI = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.GUMMEI == nil {
				return nil, false
			}

			return m.GUMMEI, true
		},
	},
	{
		ID: IDMMEUES1APID2, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID2 = &v

			return nil
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.MMEUES1APID2 == nil {
				return nil, false
			}

			return m.MMEUES1APID2, true
		},
	},
	{
		ID: IDTimeRefDistribution, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequest, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.TimeRefDistribution)
		},
		Encode: func(m *HandoverRequest) (per.Marshaler, bool) { return optionalIE(m.TimeRefDistribution) },
	},
}

func (m *HandoverRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverRequestIEs, m)
}

func (m *HandoverRequest) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverRequest(value []byte) (*HandoverRequest, error) {
	return parseMessageBody[HandoverRequest](ProcHandoverResourceAllocation, TriggeringInitiatingMessage, handoverRequestIEs, value)
}

func DecodeHandoverRequest(value []byte) (*HandoverRequest, error) {
	return decodeMessageBody[HandoverRequest](ProcHandoverResourceAllocation, handoverRequestIEs, value)
}

// TS 36.413 §9.1.5.5.
type HandoverRequestAcknowledge struct {
	MMEUES1APID            *MMEUES1APID
	ENBUES1APID            *ENBUES1APID
	ERABAdmitted           ERABAdmittedList
	ERABFailedToSetup      []ERABItem
	TargetToSource         TransparentContainer
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var handoverRequestAcknowledgeIEs = []ieSpec[HandoverRequestAcknowledge]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v ENBUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		ID: IDERABAdmittedList, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABAdmitted, err = decodeItemList[ERABAdmittedItem](r, enc, maxnoofERABs, IDERABAdmittedItem, CriticalityIgnore)

			return err
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if len(m.ERABAdmitted) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABAdmittedItem, CriticalityIgnore, m.ERABAdmitted), true
		},
	},
	{
		ID: IDERABFailedToSetupListHOReqAck, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var err error

			m.ERABFailedToSetup, err = decodeItemList[ERABItem](r, enc, maxnoofERABs, IDERABFailedtoSetupItemHOReqAck, CriticalityIgnore)

			return err
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if len(m.ERABFailedToSetup) == 0 {
				return nil, false
			}

			return containerList(maxnoofERABs, IDERABFailedtoSetupItemHOReqAck, CriticalityIgnore, m.ERABFailedToSetup), true
		},
	},
	{
		ID: IDTargetToSourceTransparentContainer, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.TargetToSource)
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.TargetToSource == nil {
				return nil, false
			}

			return &m.TargetToSource, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverRequestAcknowledge, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *HandoverRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverRequestAcknowledgeIEs, m)
}

func (m *HandoverRequestAcknowledge) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&SuccessfulOutcome{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverRequestAcknowledge(value []byte) (*HandoverRequestAcknowledge, error) {
	return parseMessageBody[HandoverRequestAcknowledge](ProcHandoverResourceAllocation, TriggeringSuccessfulOutcome, handoverRequestAcknowledgeIEs, value)
}

func DecodeHandoverRequestAcknowledge(value []byte) (*HandoverRequestAcknowledge, error) {
	return decodeMessageBody[HandoverRequestAcknowledge](ProcHandoverResourceAllocation, handoverRequestAcknowledgeIEs, value)
}

// TS 36.413 §9.1.5.6.
type HandoverFailure struct {
	MMEUES1APID            *MMEUES1APID
	Cause                  *Cause
	CriticalityDiagnostics *CriticalityDiagnostics

	messageMeta
}

var handoverFailureIEs = []ieSpec[HandoverFailure]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverFailure, r *per.Reader, enc per.Encoding) error {
			var v MMEUES1APID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		Encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverFailure, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		ID: IDCriticalityDiagnostics, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *HandoverFailure, r *per.Reader, enc per.Encoding) error {
			var v CriticalityDiagnostics

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.CriticalityDiagnostics = &v

			return nil
		},
		Encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.CriticalityDiagnostics == nil {
				return nil, false
			}

			return m.CriticalityDiagnostics, true
		},
	},
}

func (m *HandoverFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcHandoverResourceAllocation, handoverFailureIEs, m)
}

func (m *HandoverFailure) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&UnsuccessfulOutcome{
		ProcedureCode: ProcHandoverResourceAllocation,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseHandoverFailure(value []byte) (*HandoverFailure, error) {
	return parseMessageBody[HandoverFailure](ProcHandoverResourceAllocation, TriggeringUnsuccessfulOutcome, handoverFailureIEs, value)
}

func DecodeHandoverFailure(value []byte) (*HandoverFailure, error) {
	return decodeMessageBody[HandoverFailure](ProcHandoverResourceAllocation, handoverFailureIEs, value)
}
