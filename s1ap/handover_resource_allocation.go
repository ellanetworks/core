// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

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

func (m *HandoverRequest) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &m.MMEUES1APID},
		{id: idHandoverType, crit: CriticalityReject, val: &m.HandoverType},
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
		{id: idUEAggregateMaximumBitrate, crit: CriticalityReject, val: &m.UEAMBR},
		{id: idERABToBeSetupListHOReq, crit: CriticalityReject, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABToBeSetupItemHOReq, CriticalityReject, m.ERABToBeSetup)
		})},
		{id: idSourceToTargetTransparentContainer, crit: CriticalityReject, val: &m.SourceToTarget},
		{id: idUESecurityCapabilities, crit: CriticalityReject, val: &m.UESecurityCapabilities},
		{id: idSecurityContext, crit: CriticalityReject, val: &m.SecurityContext},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
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
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverRequest preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &HandoverRequest{}

	var seenMME, seenType, seenCause, seenAMBR, seenERAB, seenContainer, seenSec, seenCtx bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idHandoverType:
			err = perIEDecode(f.value, &m.HandoverType)
			seenType = true
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		case idUEAggregateMaximumBitrate:
			err = perIEDecode(f.value, &m.UEAMBR)
			seenAMBR = true
		case idERABToBeSetupListHOReq:
			m.ERABToBeSetup, err = decodeItemList[ERABToBeSetupItemHOReq](per.NewReader(f.value), enc, maxnoofERABs)
			seenERAB = true
		case idSourceToTargetTransparentContainer:
			err = perIEDecode(f.value, &m.SourceToTarget)
			seenContainer = true
		case idUESecurityCapabilities:
			err = perIEDecode(f.value, &m.UESecurityCapabilities)
			seenSec = true
		case idSecurityContext:
			err = perIEDecode(f.value, &m.SecurityContext)
			seenCtx = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverRequest IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenType || !seenCause || !seenAMBR || !seenERAB || !seenContainer || !seenSec || !seenCtx {
		return nil, fmt.Errorf("s1ap: HandoverRequest missing mandatory IE")
	}

	return m, nil
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

func (m *HandoverRequestAcknowledge) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idENBUES1APID, crit: CriticalityIgnore, val: &m.ENBUES1APID},
		{id: idERABAdmittedList, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABAdmittedItem, CriticalityIgnore, m.ERABAdmitted)
		})},
	}

	if len(m.ERABFailedToSetup) > 0 {
		fields = append(fields, ieField{id: idERABFailedToSetupListHOReqAck, crit: CriticalityIgnore, val: per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
			return encodeSingleContainerList(w, enc, maxnoofERABs, idERABItem, CriticalityIgnore, m.ERABFailedToSetup)
		})})
	}

	fields = append(fields, ieField{id: idTargetToSourceTransparentContainer, crit: CriticalityReject, val: &m.TargetToSource})

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
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
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverRequestAcknowledge preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &HandoverRequestAcknowledge{}

	var seenMME, seenENB, seenAdmitted, seenContainer bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(f.value, &m.ENBUES1APID)
			seenENB = true
		case idERABAdmittedList:
			m.ERABAdmitted, err = decodeItemList[ERABAdmittedItem](per.NewReader(f.value), enc, maxnoofERABs)
			seenAdmitted = true
		case idERABFailedToSetupListHOReqAck:
			m.ERABFailedToSetup, err = decodeItemList[ERABItem](per.NewReader(f.value), enc, maxnoofERABs)
		case idTargetToSourceTransparentContainer:
			err = perIEDecode(f.value, &m.TargetToSource)
			seenContainer = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverRequestAcknowledge IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenENB || !seenAdmitted || !seenContainer {
		return nil, fmt.Errorf("s1ap: HandoverRequestAcknowledge missing mandatory IE")
	}

	return m, nil
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

func (m *HandoverFailure) encodeBody(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityIgnore, val: &m.MMEUES1APID},
		{id: idCause, crit: CriticalityIgnore, val: &m.Cause},
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
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
	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("s1ap: HandoverFailure preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return nil, err
		}
	}

	m := &HandoverFailure{}

	var seenMME, seenCause bool

	for _, f := range fields {
		switch f.id {
		case idMMEUES1APID:
			err = perIEDecode(f.value, &m.MMEUES1APID)
			seenMME = true
		case idCause:
			err = perIEDecode(f.value, &m.Cause)
			seenCause = true
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}

		if err != nil {
			return nil, fmt.Errorf("s1ap: HandoverFailure IE %d: %w", f.id, err)
		}
	}

	if !seenMME || !seenCause {
		return nil, fmt.Errorf("s1ap: HandoverFailure missing mandatory IE")
	}

	return m, nil
}
