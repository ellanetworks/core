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
	ERABToBeSetup          []ERABToBeSetupItemHOReq
	SourceToTarget         TransparentContainer
	UESecurityCapabilities UESecurityCapabilities
	SecurityContext        SecurityContext

	messageMeta
}

var handoverRequestIEs = []ieSpec[HandoverRequest]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idHandoverType, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
	{
		id: idUEAggregateMaximumBitrate, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UEAMBR)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UEAMBR, true },
	},
	{
		id: idERABToBeSetupListHOReq, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemHOReq](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeSetupItemHOReq, CriticalityReject, m.ERABToBeSetup)
			}), true
		},
	},
	{
		id: idSourceToTargetTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceToTarget)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) {
			if m.SourceToTarget == nil {
				return nil, false
			}

			return &m.SourceToTarget, true
		},
	},
	{
		id: idUESecurityCapabilities, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UESecurityCapabilities)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UESecurityCapabilities, true },
	},
	{
		id: idSecurityContext, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SecurityContext)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.SecurityContext, true },
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

// TS 36.413 §9.1.5.5.
type HandoverRequestAcknowledge struct {
	MMEUES1APID       *MMEUES1APID
	ENBUES1APID       *ENBUES1APID
	ERABAdmitted      []ERABAdmittedItem
	ERABFailedToSetup []ERABItem
	TargetToSource    TransparentContainer

	messageMeta
}

var handoverRequestAcknowledgeIEs = []ieSpec[HandoverRequestAcknowledge]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var v ENBUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.ENBUES1APID = &v

			return nil
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.ENBUES1APID == nil {
				return nil, false
			}

			return m.ENBUES1APID, true
		},
	},
	{
		id: idERABAdmittedList, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABAdmitted, err = decodeItemList[ERABAdmittedItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if len(m.ERABAdmitted) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABAdmittedItem, CriticalityIgnore, m.ERABAdmitted)
			}), true
		},
	},
	{
		id: idERABFailedToSetupListHOReqAck, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABFailedToSetup, err = decodeItemList[ERABItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if len(m.ERABFailedToSetup) == 0 {
				return nil, false
			}

			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToSetup)
			}), true
		},
	},
	{
		id: idTargetToSourceTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetToSource)
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			if m.TargetToSource == nil {
				return nil, false
			}

			return &m.TargetToSource, true
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

// TS 36.413 §9.1.5.6.
type HandoverFailure struct {
	MMEUES1APID *MMEUES1APID
	Cause       *Cause

	messageMeta
}

var handoverFailureIEs = []ieSpec[HandoverFailure]{
	{
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverFailure, raw []byte, enc per.Encoding) error {
			var v MMEUES1APID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.MMEUES1APID = &v

			return nil
		},
		encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.MMEUES1APID == nil {
				return nil, false
			}

			return m.MMEUES1APID, true
		},
	},
	{
		id: idCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverFailure, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *HandoverFailure) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
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
