// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

type OverloadAction uint8

const (
	OverloadActionRejectNonEmergencyMODT OverloadAction = iota
	OverloadActionRejectRRCCRSignalling
	OverloadActionPermitEmergencySessionsAndMTServicesOnly
	OverloadActionPermitHighPrioritySessionsAndMTServicesOnly

	overloadActionRootCount = 4
)

func (a OverloadAction) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, overloadActionRootCount, int64(a), "OverloadAction")
}

func (a *OverloadAction) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeRootEnumerated(r, enc, overloadActionRootCount, "OverloadAction")
	if err != nil {
		return err
	}

	*a = OverloadAction(v)

	return nil
}

const (
	overloadResponseOverloadAction = iota
	overloadResponseChoiceExtensions

	overloadResponseAlternatives = 2
)

type OverloadResponse struct {
	OverloadAction OverloadAction
}

func (o OverloadResponse) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, overloadResponseAlternatives-1, overloadResponseOverloadAction); err != nil {
		return err
	}

	return o.OverloadAction.MarshalPER(w, enc)
}

func (o *OverloadResponse) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, overloadResponseAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: OverloadResponse choice: %w", err)
	}

	if idx == overloadResponseChoiceExtensions {
		return decodeChoiceExtension(r, enc, "OverloadResponse")
	}

	return o.OverloadAction.UnmarshalPER(r, enc)
}

const (
	trafficLoadReductionMin = 1
	trafficLoadReductionMax = 99
)

type TrafficLoadReductionIndication uint8

func (t TrafficLoadReductionIndication) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: trafficLoadReductionMin, HasLB: true, UB: trafficLoadReductionMax, HasUB: true}, int64(t))
}

func (t *TrafficLoadReductionIndication) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: trafficLoadReductionMin, HasLB: true, UB: trafficLoadReductionMax, HasUB: true})
	if err != nil {
		return err
	}

	*t = TrafficLoadReductionIndication(v)

	return nil
}

type SliceOverloadItem struct {
	_      [0]struct{} `per:"extseq"`
	SNSSAI SNSSAI
	_      ieExtensions `per:",skip"`
}

type OverloadStartNSSAIItem struct {
	_                                   [0]struct{}                     `per:"extseq"`
	SliceOverloadList                   []SliceOverloadItem             `per:"SEQUENCE-OF,size:1..1024"`
	SliceOverloadResponse               *OverloadResponse               `per:",optional"`
	SliceTrafficLoadReductionIndication *TrafficLoadReductionIndication `per:",optional"`
	_                                   ieExtensions                    `per:",skip"`
}

type OverloadStartNSSAIList []OverloadStartNSSAIItem

func (l OverloadStartNSSAIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofSliceItems, []OverloadStartNSSAIItem(l))
}

func (l *OverloadStartNSSAIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[OverloadStartNSSAIItem](r, enc, 1, maxnoofSliceItems)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type OverloadStart struct {
	AMFOverloadResponse               *OverloadResponse
	AMFTrafficLoadReductionIndication *TrafficLoadReductionIndication
	OverloadStartNSSAIList            OverloadStartNSSAIList

	messageMeta
}

var overloadStartIEs = []ieSpec[OverloadStart]{
	{
		id: IDAMFOverloadResponse, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *OverloadStart, raw []byte, enc per.Encoding) error {
			var v OverloadResponse

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFOverloadResponse = &v

			return nil
		},
		encode: func(m *OverloadStart) (per.Marshaler, bool) {
			if m.AMFOverloadResponse == nil {
				return nil, false
			}

			return m.AMFOverloadResponse, true
		},
	},
	{
		id: IDAMFTrafficLoadReductionIndication, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *OverloadStart, raw []byte, enc per.Encoding) error {
			var v TrafficLoadReductionIndication

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFTrafficLoadReductionIndication = &v

			return nil
		},
		encode: func(m *OverloadStart) (per.Marshaler, bool) {
			if m.AMFTrafficLoadReductionIndication == nil {
				return nil, false
			}

			return m.AMFTrafficLoadReductionIndication, true
		},
	},
	{
		id: IDOverloadStartNSSAIList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *OverloadStart, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.OverloadStartNSSAIList)
		},
		encode: func(m *OverloadStart) (per.Marshaler, bool) {
			if len(m.OverloadStartNSSAIList) == 0 {
				return nil, false
			}

			return m.OverloadStartNSSAIList, true
		},
	},
}

func (m *OverloadStart) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcOverloadStart, overloadStartIEs, m)
}

func (m *OverloadStart) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcOverloadStart,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseOverloadStart(value []byte) (*OverloadStart, error) {
	return parseMessageBody[OverloadStart](ProcOverloadStart, TriggeringInitiatingMessage, overloadStartIEs, value)
}

type OverloadStop struct {
	messageMeta
}

var overloadStopIEs = []ieSpec[OverloadStop]{}

func (m *OverloadStop) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcOverloadStop, overloadStopIEs, m)
}

func (m *OverloadStop) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcOverloadStop,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseOverloadStop(value []byte) (*OverloadStop, error) {
	return parseMessageBody[OverloadStop](ProcOverloadStop, TriggeringInitiatingMessage, overloadStopIEs, value)
}
