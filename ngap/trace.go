// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
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
	TraceDepthMinimumOnlyVendorSpecificTraceRecord
	TraceDepthMediumOnlyVendorSpecificTraceRecord
	TraceDepthMaximumOnlyVendorSpecificTraceRecord
)

type InterfacesToTrace uint8

const (
	InterfaceToTraceNGC InterfacesToTrace = 1 << (7 - iota)
	InterfaceToTraceXnC
	InterfaceToTraceUu
	InterfaceToTraceF1C
	InterfaceToTraceE1
)

func (i InterfacesToTrace) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeBitString(w, enc, 8, 8, true, true, false, []byte{byte(i)}, 8)
}

func (i *InterfacesToTrace) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, _, err := per.DecodeBitString(r, enc, 8, 8, true, true, false)
	if err != nil {
		return err
	}

	*i = InterfacesToTrace(b[0])

	return nil
}

type URIAddress string

func (u URIAddress) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeKnownMultiplierString(w, enc, per.CharVisibleString, 0, 0, true, false, false, string(u))
}

func (u *URIAddress) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	s, err := per.DecodeKnownMultiplierString(r, enc, per.CharVisibleString, 0, 0, true, false, false)
	*u = URIAddress(s)

	return err
}

type TraceActivation struct {
	NGRANTraceID                   NGRANTraceID
	InterfacesToTrace              InterfacesToTrace
	TraceDepth                     TraceDepth
	TraceCollectionEntityIPAddress TransportLayerAddress
	MDTConfiguration               *MDTConfiguration      `per:",extie:IDMDTConfiguration,crit:ignore"`
	TraceCollectionEntityURI       *URIAddress            `per:",extie:IDTraceCollectionEntityURI,crit:ignore"`
	iEExtensions                   extensionMeta          `per:",exttable"`
	extensions                     per.ExtensionAdditions `per:",extraw"`
}

type TraceStart struct {
	AMFUENGAPID     AMFUENGAPID
	RANUENGAPID     RANUENGAPID
	TraceActivation *TraceActivation

	messageMeta
}

var traceStartIEs = []ieSpec[TraceStart]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *TraceStart, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *TraceStart) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *TraceStart, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *TraceStart) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
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
	AMFUENGAPID  AMFUENGAPID
	RANUENGAPID  RANUENGAPID
	NGRANTraceID *NGRANTraceID
	Cause        *Cause

	messageMeta
}

var traceFailureIndicationIEs = []ieSpec[TraceFailureIndication]{
	{
		ID: IDAMFUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *TraceFailureIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.AMFUENGAPID)
		},
		Encode: func(m *TraceFailureIndication) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		ID: IDRANUENGAPID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *TraceFailureIndication, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.RANUENGAPID)
		},
		Encode: func(m *TraceFailureIndication) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		ID: IDNGRANTraceID, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *TraceFailureIndication, r *per.Reader, enc per.Encoding) error {
			var v NGRANTraceID

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.NGRANTraceID = &v

			return nil
		},
		Encode: func(m *TraceFailureIndication) (per.Marshaler, bool) {
			if m.NGRANTraceID == nil {
				return nil, false
			}

			return m.NGRANTraceID, true
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

func m1IncludesBeamMeasurements(c *M1Configuration) bool {
	return c.IncludeBeamMeasurementsIndication != nil &&
		*c.IncludeBeamMeasurementsIndication == IncludeBeamMeasurementsIndicationTrue
}
