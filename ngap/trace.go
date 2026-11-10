// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

type NGRANTraceID [8]byte

func (t NGRANTraceID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 8, 8, true, true, false, t[:])
}

func (t *NGRANTraceID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
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

const traceDepthRootValues = 6

const (
	InterfaceToTraceNGC uint8 = 1 << (7 - iota)
	InterfaceToTraceXnC
	InterfaceToTraceUu
	InterfaceToTraceF1C
	InterfaceToTraceE1
)

type TraceActivation struct {
	NGRANTraceID                   NGRANTraceID
	InterfacesToTrace              uint8
	TraceDepth                     TraceDepth
	TraceCollectionEntityIPAddress TransportLayerAddress
}

func (t TraceActivation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)
	w.WriteBit(false)

	if err := t.NGRANTraceID.MarshalPER(w, enc); err != nil {
		return err
	}

	if err := per.EncodeBitString(w, enc, 8, 8, true, true, false, []byte{t.InterfacesToTrace}, 8); err != nil {
		return err
	}

	if err := encodeRootEnumerated(w, enc, traceDepthRootValues, int64(t.TraceDepth), "TraceDepth"); err != nil {
		return err
	}

	if len(t.TraceCollectionEntityIPAddress) == 0 {
		return fmt.Errorf("ngap: TraceActivation without a Trace Collection Entity IP Address")
	}

	return t.TraceCollectionEntityIPAddress.MarshalPER(w, enc)
}

func (t *TraceActivation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	var v TraceActivation

	if err := v.NGRANTraceID.UnmarshalPER(r, enc); err != nil {
		return err
	}

	b, _, err := per.DecodeBitString(r, enc, 8, 8, true, true, false)
	if err != nil {
		return err
	}

	v.InterfacesToTrace = b[0]

	depth, err := decodeRootEnumerated(r, enc, traceDepthRootValues, "TraceDepth")
	if err != nil {
		return err
	}

	v.TraceDepth = TraceDepth(depth)

	if err := v.TraceCollectionEntityIPAddress.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*t = v

	return nil
}

type TraceStart struct {
	AMFUENGAPID     AMFUENGAPID
	RANUENGAPID     RANUENGAPID
	TraceActivation *TraceActivation

	messageMeta
}

var traceStartIEs = []ieSpec[TraceStart]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *TraceStart, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *TraceStart) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *TraceStart, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *TraceStart) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDTraceActivation, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *TraceStart, raw []byte, enc per.Encoding) error {
			var v TraceActivation

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.TraceActivation = &v

			return nil
		},
		encode: func(m *TraceStart) (per.Marshaler, bool) {
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

type TraceFailureIndication struct {
	AMFUENGAPID  AMFUENGAPID
	RANUENGAPID  RANUENGAPID
	NGRANTraceID *NGRANTraceID
	Cause        *Cause

	messageMeta
}

var traceFailureIndicationIEs = []ieSpec[TraceFailureIndication]{
	{
		id: IDAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *TraceFailureIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *TraceFailureIndication) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: IDRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *TraceFailureIndication, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *TraceFailureIndication) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: IDNGRANTraceID, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *TraceFailureIndication, raw []byte, enc per.Encoding) error {
			var v NGRANTraceID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.NGRANTraceID = &v

			return nil
		},
		encode: func(m *TraceFailureIndication) (per.Marshaler, bool) {
			if m.NGRANTraceID == nil {
				return nil, false
			}

			return m.NGRANTraceID, true
		},
	},
	{
		id: IDCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *TraceFailureIndication, raw []byte, enc per.Encoding) error {
			var v Cause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.Cause = &v

			return nil
		},
		encode: func(m *TraceFailureIndication) (per.Marshaler, bool) {
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
