// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

type OverloadAction uint8

const (
	OverloadActionRejectNonEmergencyMODT OverloadAction = iota
	OverloadActionRejectRRCCRSignalling
	OverloadActionPermitEmergencySessionsAndMTServicesOnly
	OverloadActionPermitHighPrioritySessionsAndMTServicesOnly
)

type OverloadResponse struct {
	OverloadAction  *OverloadAction  `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
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
	extensions   per.ExtensionAdditions `per:",extraw"`
	SNSSAI       SNSSAI
	iEExtensions extensionMeta `per:",exttable"`
}

type OverloadStartNSSAIItem struct {
	extensions                          per.ExtensionAdditions          `per:",extraw"`
	SliceOverloadList                   []SliceOverloadItem             `per:"SEQUENCE-OF,size:1..1024"`
	SliceOverloadResponse               *OverloadResponse               `per:",optional"`
	SliceTrafficLoadReductionIndication *TrafficLoadReductionIndication `per:",optional"`
	iEExtensions                        extensionMeta                   `per:",exttable"`
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

const maxnoofGUAMIs = 1024

type GUAMIListItem struct {
	GUAMI        GUAMI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type GUAMIList []GUAMIListItem

func (l GUAMIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofGUAMIs, []GUAMIListItem(l))
}

func (l *GUAMIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[GUAMIListItem](r, enc, 1, maxnoofGUAMIs)
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
	GUAMIList                         GUAMIList

	messageMeta
}

var overloadStartIEs = []ieSpec[OverloadStart]{
	{
		ID: IDAMFOverloadResponse, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *OverloadStart, r *per.Reader, enc per.Encoding) error {
			var v OverloadResponse

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFOverloadResponse = &v

			return nil
		},
		Encode: func(m *OverloadStart) (per.Marshaler, bool) {
			if m.AMFOverloadResponse == nil {
				return nil, false
			}

			return m.AMFOverloadResponse, true
		},
	},
	{
		ID: IDAMFTrafficLoadReductionIndication, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *OverloadStart, r *per.Reader, enc per.Encoding) error {
			var v TrafficLoadReductionIndication

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.AMFTrafficLoadReductionIndication = &v

			return nil
		},
		Encode: func(m *OverloadStart) (per.Marshaler, bool) {
			if m.AMFTrafficLoadReductionIndication == nil {
				return nil, false
			}

			return m.AMFTrafficLoadReductionIndication, true
		},
	},
	{
		ID: IDOverloadStartNSSAIList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *OverloadStart, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.OverloadStartNSSAIList)
		},
		Encode: func(m *OverloadStart) (per.Marshaler, bool) {
			if len(m.OverloadStartNSSAIList) == 0 {
				return nil, false
			}

			return m.OverloadStartNSSAIList, true
		},
	},
	{
		ID: IDGUAMIList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *OverloadStart, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GUAMIList)
		},
		Encode: func(m *OverloadStart) (per.Marshaler, bool) {
			if len(m.GUAMIList) == 0 {
				return nil, false
			}

			return m.GUAMIList, true
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

func DecodeOverloadStart(value []byte) (*OverloadStart, error) {
	return decodeMessageBody[OverloadStart](ProcOverloadStart, overloadStartIEs, value)
}

type OverloadStop struct {
	GUAMIList GUAMIList

	messageMeta
}

var overloadStopIEs = []ieSpec[OverloadStop]{
	{
		ID: IDGUAMIList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *OverloadStop, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.GUAMIList)
		},
		Encode: func(m *OverloadStop) (per.Marshaler, bool) {
			if len(m.GUAMIList) == 0 {
				return nil, false
			}

			return m.GUAMIList, true
		},
	},
}

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

func DecodeOverloadStop(value []byte) (*OverloadStop, error) {
	return decodeMessageBody[OverloadStop](ProcOverloadStop, overloadStopIEs, value)
}
