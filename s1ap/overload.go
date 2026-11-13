// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

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

	overloadActionRootCount  = 3
	overloadActionValueCount = 7
)

type OverloadResponse struct {
	OverloadAction OverloadAction
}

func (o OverloadResponse) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if o.OverloadAction >= overloadActionValueCount {
		return fmt.Errorf("s1ap: OverloadAction %d is not defined", o.OverloadAction)
	}

	w.WriteBit(false)

	return per.EncodeEnumerated(w, enc, overloadActionRootCount, true, int64(o.OverloadAction))
}

func (o *OverloadResponse) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("s1ap: OverloadResponse choice: %w", err)
	}

	if isExt {
		return fmt.Errorf("%w: OverloadResponse extension alternative", errNotComprehended)
	}

	v, err := per.DecodeEnumerated(r, enc, overloadActionRootCount, true)
	if err != nil {
		return fmt.Errorf("s1ap: OverloadAction: %w", err)
	}

	if v >= overloadActionValueCount {
		return fmt.Errorf("%w: OverloadAction extension value %d", errNotComprehended, v)
	}

	*o = OverloadResponse{OverloadAction: OverloadAction(v)}

	return nil
}

func decodeGUMMEIList(raw []byte, enc per.Encoding) ([]GUMMEI, error) {
	return unmarshalSeqOf[GUMMEI](per.NewReader(raw), enc, 1, maxnoofMMECs)
}

func gUMMEIListIE(l []GUMMEI) (per.Marshaler, bool) {
	if len(l) == 0 {
		return nil, false
	}

	return per.MarshalerFunc(func(w *per.Writer, enc per.Encoding) error {
		return marshalSeqOf(w, enc, 1, maxnoofMMECs, l)
	}), true
}

type OverloadStart struct {
	OverloadResponse               OverloadResponse
	GUMMEIList                     []GUMMEI
	TrafficLoadReductionIndication *uint8

	messageMeta
}

var overloadStartIEs = []ieSpec[OverloadStart]{
	{
		id: IDOverloadResponse, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *OverloadStart, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.OverloadResponse)
		},
		encode: func(m *OverloadStart) (per.Marshaler, bool) { return &m.OverloadResponse, true },
	},
	{
		id: IDGUMMEIList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *OverloadStart, raw []byte, enc per.Encoding) error {
			var err error

			m.GUMMEIList, err = decodeGUMMEIList(raw, enc)

			return err
		},
		encode: func(m *OverloadStart) (per.Marshaler, bool) { return gUMMEIListIE(m.GUMMEIList) },
	},
	{
		id: IDTrafficLoadReductionIndication, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *OverloadStart, raw []byte, enc per.Encoding) error {
			return decodeOptionalInteger(raw, trafficLoadReductionIndicationBounds, &m.TrafficLoadReductionIndication)
		},
		encode: func(m *OverloadStart) (per.Marshaler, bool) {
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

type OverloadStop struct {
	GUMMEIList []GUMMEI

	messageMeta
}

var overloadStopIEs = []ieSpec[OverloadStop]{
	{
		id: IDGUMMEIList, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *OverloadStop, raw []byte, enc per.Encoding) error {
			var err error

			m.GUMMEIList, err = decodeGUMMEIList(raw, enc)

			return err
		},
		encode: func(m *OverloadStop) (per.Marshaler, bool) { return gUMMEIListIE(m.GUMMEIList) },
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
