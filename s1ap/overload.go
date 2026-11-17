// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

var trafficLoadReductionIndicationBounds = per.Bounds{LB: 1, UB: 99, HasLB: true, HasUB: true}

type OverloadAction uint8

const (
	OverloadActionRejectNonEmergencyMODT OverloadAction = iota
	OverloadActionRejectRRCCRSignalling
	OverloadActionPermitEmergencySessionsAndMobileTerminatedServicesOnly
	OverloadActionPermitHighPrioritySessionsAndMobileTerminatedServicesOnly
	OverloadActionRejectDelayTolerantAccess
	OverloadActionPermitHighPrioritySessionsAndExceptionReportingAndMobileTerminatedServicesOnly
	OverloadActionNotAcceptMODataOrDelayTolerantAccessFromCPCIoT
)

type OverloadResponse struct {
	OverloadAction *OverloadAction           `per:",choice:0"`
	Ext            *per.ExtensionAlternative `per:",extalt"`
}

func decodeGUMMEIList(r *per.Reader, enc per.Encoding) ([]GUMMEI, error) {
	return unmarshalSeqOf[GUMMEI](r, enc, 1, maxnoofMMECs)
}

func gUMMEIListIE(l []GUMMEI) (per.Marshaler, bool) {
	if len(l) == 0 {
		return nil, false
	}

	return seqOf(1, maxnoofMMECs, &l), true
}

type OverloadStart struct {
	OverloadResponse               OverloadResponse
	GUMMEIList                     []GUMMEI
	TrafficLoadReductionIndication *uint8

	messageMeta
}

var overloadStartIEs = []ieSpec[OverloadStart]{
	{
		ID: IDOverloadResponse, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *OverloadStart, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.OverloadResponse)
		},
		Encode: func(m *OverloadStart) (per.Marshaler, bool) { return &m.OverloadResponse, true },
	},
	{
		ID: IDGUMMEIList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *OverloadStart, r *per.Reader, enc per.Encoding) error {
			var err error

			m.GUMMEIList, err = decodeGUMMEIList(r, enc)

			return err
		},
		Encode: func(m *OverloadStart) (per.Marshaler, bool) { return gUMMEIListIE(m.GUMMEIList) },
	},
	{
		ID: IDTrafficLoadReductionIndication, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *OverloadStart, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalInteger(r, trafficLoadReductionIndicationBounds, &m.TrafficLoadReductionIndication)
		},
		Encode: func(m *OverloadStart) (per.Marshaler, bool) {
			if m.TrafficLoadReductionIndication == nil {
				return nil, false
			}

			return integerIE(trafficLoadReductionIndicationBounds, int64(*m.TrafficLoadReductionIndication)), true
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
	GUMMEIList []GUMMEI

	messageMeta
}

var overloadStopIEs = []ieSpec[OverloadStop]{
	{
		ID: IDGUMMEIList, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *OverloadStop, r *per.Reader, enc per.Encoding) error {
			var err error

			m.GUMMEIList, err = decodeGUMMEIList(r, enc)

			return err
		},
		Encode: func(m *OverloadStop) (per.Marshaler, bool) { return gUMMEIListIE(m.GUMMEIList) },
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
