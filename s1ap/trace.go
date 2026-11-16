// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

type EUTRANTraceID [8]byte

func (t EUTRANTraceID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 8, 8, true, true, false, t[:])
}

func (t *EUTRANTraceID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 8, 8, true, true, false)
	if err != nil {
		return err
	}

	copy(t[:], b)

	return nil
}

type TraceDepth uint8

const (
	TraceDepthMinimum TraceDepth = iota
	TraceDepthMedium
	TraceDepthMaximum
	TraceDepthMinimumWithoutVendorSpecificExtension
	TraceDepthMediumWithoutVendorSpecificExtension
	TraceDepthMaximumWithoutVendorSpecificExtension
)

type InterfacesToTrace uint8

const (
	InterfaceToTraceS1MME InterfacesToTrace = 1 << (7 - iota)
	InterfaceToTraceX2
	InterfaceToTraceUu
	InterfaceToTraceF1C
	InterfaceToTraceE1
)

func (i InterfacesToTrace) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(i), 8)
}

func (i *InterfacesToTrace) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, 8)
	*i = InterfacesToTrace(v)

	return err
}

type TraceActivation struct {
	EUTRANTraceID                  EUTRANTraceID
	InterfacesToTrace              InterfacesToTrace
	TraceDepth                     TraceDepth
	TraceCollectionEntityIPAddress TransportLayerAddress
	MDTConfigurationNR             *MDTConfigurationNR    `per:",extie:IDMDTConfigurationNR,crit:ignore"`
	TraceCollectionEntityURI       *URIAddress            `per:",extie:IDTraceCollectionEntityURI,crit:ignore"`
	iEExtensions                   extensionMeta          `per:",exttable"`
	extensions                     per.ExtensionAdditions `per:",extraw"`
}

type TraceStart struct {
	MMEUES1APID     MMEUES1APID
	ENBUES1APID     ENBUES1APID
	TraceActivation *TraceActivation

	messageMeta
}

var traceStartIEs = []ieSpec[TraceStart]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *TraceStart, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *TraceStart) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *TraceStart, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *TraceStart) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDTraceActivation, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *TraceStart, r *per.Reader, enc per.Encoding) error {
			var v TraceActivation

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TraceActivation = &v

			return nil
		},
		Encode: func(m *TraceStart) (per.Marshaler, bool) {
			if m.TraceActivation == nil {
				return nil, false
			}

			return m.TraceActivation, true
		},
	},
}

func (m *TraceStart) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcTraceStart, traceStartIEs, m)
}

func (m *TraceStart) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcTraceStart,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseTraceStart(value []byte) (*TraceStart, error) {
	return parseMessageBody[TraceStart](ProcTraceStart, TriggeringInitiatingMessage, traceStartIEs, value)
}

func DecodeTraceStart(value []byte) (*TraceStart, error) {
	return decodeMessageBody[TraceStart](ProcTraceStart, traceStartIEs, value)
}

type TraceFailureIndication struct {
	MMEUES1APID   MMEUES1APID
	ENBUES1APID   ENBUES1APID
	EUTRANTraceID *EUTRANTraceID
	Cause         *Cause

	messageMeta
}

var traceFailureIndicationIEs = []ieSpec[TraceFailureIndication]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *TraceFailureIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *TraceFailureIndication) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *TraceFailureIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *TraceFailureIndication) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDEUTRANTraceID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *TraceFailureIndication, r *per.Reader, enc per.Encoding) error {
			var v EUTRANTraceID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.EUTRANTraceID = &v

			return nil
		},
		Encode: func(m *TraceFailureIndication) (per.Marshaler, bool) {
			if m.EUTRANTraceID == nil {
				return nil, false
			}

			return m.EUTRANTraceID, true
		},
	},
	{
		ID: IDCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *TraceFailureIndication, r *per.Reader, enc per.Encoding) error {
			var v Cause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		Encode: func(m *TraceFailureIndication) (per.Marshaler, bool) {
			if m.Cause == nil {
				return nil, false
			}

			return m.Cause, true
		},
	},
}

func (m *TraceFailureIndication) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcTraceFailureIndication, traceFailureIndicationIEs, m)
}

func (m *TraceFailureIndication) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcTraceFailureIndication,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseTraceFailureIndication(value []byte) (*TraceFailureIndication, error) {
	return parseMessageBody[TraceFailureIndication](ProcTraceFailureIndication, TriggeringInitiatingMessage, traceFailureIndicationIEs, value)
}

func DecodeTraceFailureIndication(value []byte) (*TraceFailureIndication, error) {
	return decodeMessageBody[TraceFailureIndication](ProcTraceFailureIndication, traceFailureIndicationIEs, value)
}
