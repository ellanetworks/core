// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// HandoverRequest is the HANDOVER REQUEST message (TS 36.413), sent by
// the MME to the target eNB to reserve resources for an incoming handover. It
// carries no eNB UE S1AP ID: the target eNB allocates its own and returns it in
// the HANDOVER REQUEST ACKNOWLEDGE. SecurityContext carries the {NCC, NH} the
// target uses to derive KeNB (TS 33.401); SourceToTarget is the opaque
// source-to-target transparent container.
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

// handoverRequestIEs is the HandoverRequest IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseHandoverRequest decodes the message from an initiatingMessage open-type
// payload.
func ParseHandoverRequest(value []byte) (*HandoverRequest, error) {
	return parseMessageBody[HandoverRequest](ProcHandoverResourceAllocation, handoverRequestIEs, value)
}

// HandoverRequestAcknowledge is the HANDOVER REQUEST ACKNOWLEDGE message
// (TS 36.413), the successful outcome the target eNB returns. ERABAdmitted
// carries the target eNB's S1-U downlink endpoint per E-RAB; ERABFailedToSetup
// lists the bearers the target rejected; TargetToSource is the opaque target-to-
// source transparent container.
type HandoverRequestAcknowledge struct {
	MMEUES1APID       MMEUES1APID
	ENBUES1APID       ENBUES1APID
	ERABAdmitted      []ERABAdmittedItem
	ERABFailedToSetup []ERABItem
	TargetToSource    TransparentContainer

	unmodeledIEs
}

// handoverRequestAcknowledgeIEs is the HandoverRequestAcknowledge IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseHandoverRequestAcknowledge decodes the message from a successfulOutcome
// open-type payload.
func ParseHandoverRequestAcknowledge(value []byte) (*HandoverRequestAcknowledge, error) {
	return parseMessageBody[HandoverRequestAcknowledge](ProcHandoverResourceAllocation, handoverRequestAcknowledgeIEs, value)
}

// HandoverFailure is the HANDOVER FAILURE message (TS 36.413 in the
// Handover Resource Allocation procedure), the unsuccessful outcome the target eNB
// returns when it cannot admit the handover. It carries no eNB UE S1AP ID: the
// target allocated no UE context.
type HandoverFailure struct {
	MMEUES1APID MMEUES1APID
	Cause       Cause

	unmodeledIEs
}

// handoverFailureIEs is the HandoverFailure IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseHandoverFailure decodes the message from an unsuccessfulOutcome open-type
// payload.
func ParseHandoverFailure(value []byte) (*HandoverFailure, error) {
	return parseMessageBody[HandoverFailure](ProcHandoverResourceAllocation, handoverFailureIEs, value)
}
