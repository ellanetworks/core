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
	Cause                  Cause
	UEAMBR                 UEAggregateMaximumBitRate
	ERABToBeSetup          []ERABToBeSetupItemHOReq
	SourceToTarget         TransparentContainer
	UESecurityCapabilities UESecurityCapabilities
	SecurityContext        SecurityContext

	unmodeledIEs
}

var handoverRequestIEs = []ieSpec[HandoverRequest]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idHandoverType, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.HandoverType)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.HandoverType, true },
	},
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.Cause, true },
	},
	{
		id: idUEAggregateMaximumBitrate, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UEAMBR)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UEAMBR, true },
	},
	{
		id: idERABToBeSetupListHOReq, presence: PresenceMandatory, crit: CriticalityReject,
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
		id: idSourceToTargetTransparentContainer, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SourceToTarget)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.SourceToTarget, true },
	},
	{
		id: idUESecurityCapabilities, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UESecurityCapabilities)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.UESecurityCapabilities, true },
	},
	{
		id: idSecurityContext, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequest, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.SecurityContext)
		},
		encode: func(m *HandoverRequest) (per.Marshaler, bool) { return &m.SecurityContext, true },
	},
}

func (m *HandoverRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverRequestIEs, m)
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
	return parseMessageBody[HandoverRequest](ProcHandoverResourceAllocation, handoverRequestIEs, value)
}

// TS 36.413 §9.1.5.5.
type HandoverRequestAcknowledge struct {
	MMEUES1APID       MMEUES1APID
	ENBUES1APID       ENBUES1APID
	ERABAdmitted      []ERABAdmittedItem
	ERABFailedToSetup []ERABItem
	TargetToSource    TransparentContainer

	unmodeledIEs
}

var handoverRequestAcknowledgeIEs = []ieSpec[HandoverRequestAcknowledge]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idERABAdmittedList, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			var err error

			m.ERABAdmitted, err = decodeItemList[ERABAdmittedItem](per.NewReader(raw), enc, maxnoofERABs)

			return err
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) {
			return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
				return encodeSingleContainerList(w, enc, maxnoofERABs, idERABAdmittedItem, CriticalityIgnore, m.ERABAdmitted)
			}), true
		},
	},
	{
		id: idERABFailedToSetupListHOReqAck, presence: PresenceOptional, crit: CriticalityIgnore,
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
		id: idTargetToSourceTransparentContainer, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *HandoverRequestAcknowledge, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TargetToSource)
		},
		encode: func(m *HandoverRequestAcknowledge) (per.Marshaler, bool) { return &m.TargetToSource, true },
	},
}

func (m *HandoverRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverRequestAcknowledgeIEs, m)
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
	return parseMessageBody[HandoverRequestAcknowledge](ProcHandoverResourceAllocation, handoverRequestAcknowledgeIEs, value)
}

// TS 36.413 §9.1.5.6.
type HandoverFailure struct {
	MMEUES1APID MMEUES1APID
	Cause       Cause

	unmodeledIEs
}

var handoverFailureIEs = []ieSpec[HandoverFailure]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *HandoverFailure) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *HandoverFailure, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.Cause)
		},
		encode: func(m *HandoverFailure) (per.Marshaler, bool) { return &m.Cause, true },
	},
}

func (m *HandoverFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, handoverFailureIEs, m)
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
	return parseMessageBody[HandoverFailure](ProcHandoverResourceAllocation, handoverFailureIEs, value)
}
