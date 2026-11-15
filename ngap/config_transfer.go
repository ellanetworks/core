// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TargetRANNodeIDSON ::= SEQUENCE { globalRANNodeID, selectedTAI,
// iE-Extensions OPTIONAL } (extensible) — a field of the SON Configuration
// Transfer of TS 38.413 §9.3.3.6, with no standalone clause of its own.
//
// The plain TargetRANNodeID has the same shape but a different extension
// container.
type TargetRANNodeIDSON struct {
	extensions      per.ExtensionAdditions `per:",extraw"`
	GlobalRANNodeID GlobalRANNodeID
	SelectedTAI     TAI
	NRCGI           *NRCGI        `per:",extie:IDNRCGI,crit:ignore"`
	iEExtensions    extensionMeta `per:",exttable"`
}

// SONConfigurationTransfer is held as raw open-type bytes: it is relayed
// verbatim, and only the leading Target RAN Node ID is decoded
// (TS 38.413 §9.3.3.6).
//
// §8.8.1.2 has the AMF transfer it "transparently", so the interior is never
// re-encoded.
type SONConfigurationTransfer []byte

func (c SONConfigurationTransfer) MarshalPER(w *per.Writer, _ per.Encoding) error {
	return w.WriteOctets(c)
}

// The extension bit, then a presence bit each for xnTNLConfigurationInfo and
// iE-Extensions.
const sonConfigurationTransferPreambleBits = 3

// TS 38.413 §9.3.3.6.
func (c SONConfigurationTransfer) TargetRANNodeID() (TargetRANNodeIDSON, error) {
	r := per.NewReader(c)

	for range sonConfigurationTransferPreambleBits {
		if _, err := r.ReadBit(); err != nil {
			return TargetRANNodeIDSON{}, &TransferSyntaxError{Protocol: profile.Name, Err: fmt.Errorf("SONConfigurationTransfer preamble: %w", err)}
		}
	}

	var t TargetRANNodeIDSON
	if err := t.UnmarshalPER(r, per.Aligned); err != nil {
		return TargetRANNodeIDSON{}, &TransferSyntaxError{Protocol: profile.Name, Err: fmt.Errorf("SONConfigurationTransfer targetRANNodeID: %w", err)}
	}

	return t, nil
}

// TS 38.413 §9.2.7.1.
type UplinkRANConfigurationTransfer struct {
	SONConfigurationTransfer              SONConfigurationTransfer
	ENDCSONConfigurationTransferUL        *ENDCSONConfigurationTransfer
	IntersystemSONConfigurationTransferUL *IntersystemSONConfigurationTransfer

	messageMeta
}

var uplinkRANConfigurationTransferIEs = []ieSpec[UplinkRANConfigurationTransfer]{
	{
		ID: IDSONConfigurationTransferUL, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UplinkRANConfigurationTransfer, r *per.Reader, enc per.Encoding) error {
			m.SONConfigurationTransfer = SONConfigurationTransfer(restOctets(r))
			return nil
		},
		Encode: func(m *UplinkRANConfigurationTransfer) (per.Marshaler, bool) {
			if m.SONConfigurationTransfer == nil {
				return nil, false
			}

			return m.SONConfigurationTransfer, true
		},
	},
	apimpl.OptionalIE(IDENDCSONConfigurationTransferUL, CriticalityIgnore, func(m *UplinkRANConfigurationTransfer) **ENDCSONConfigurationTransfer {
		return &m.ENDCSONConfigurationTransferUL
	}),
	apimpl.OptionalIE(IDIntersystemSONConfigurationTransferUL, CriticalityIgnore, func(m *UplinkRANConfigurationTransfer) **IntersystemSONConfigurationTransfer {
		return &m.IntersystemSONConfigurationTransferUL
	}),
}

func (m *UplinkRANConfigurationTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkRANConfigurationTransfer, uplinkRANConfigurationTransferIEs, m)
}

func (m *UplinkRANConfigurationTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkRANConfigurationTransfer,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUplinkRANConfigurationTransfer(value []byte) (*UplinkRANConfigurationTransfer, error) {
	return parseMessageBody[UplinkRANConfigurationTransfer](ProcUplinkRANConfigurationTransfer, TriggeringInitiatingMessage, uplinkRANConfigurationTransferIEs, value)
}

func DecodeUplinkRANConfigurationTransfer(value []byte) (*UplinkRANConfigurationTransfer, error) {
	return decodeMessageBody[UplinkRANConfigurationTransfer](ProcUplinkRANConfigurationTransfer, uplinkRANConfigurationTransferIEs, value)
}

// TS 38.413 §9.2.7.2.
type DownlinkRANConfigurationTransfer struct {
	SONConfigurationTransfer              SONConfigurationTransfer
	ENDCSONConfigurationTransferDL        *ENDCSONConfigurationTransfer
	IntersystemSONConfigurationTransferDL *IntersystemSONConfigurationTransfer

	messageMeta
}

// The IE is optional, so an empty container is a valid message.
var downlinkRANConfigurationTransferIEs = []ieSpec[DownlinkRANConfigurationTransfer]{
	{
		ID: IDSONConfigurationTransferDL, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *DownlinkRANConfigurationTransfer, r *per.Reader, enc per.Encoding) error {
			m.SONConfigurationTransfer = SONConfigurationTransfer(restOctets(r))
			return nil
		},
		Encode: func(m *DownlinkRANConfigurationTransfer) (per.Marshaler, bool) {
			if m.SONConfigurationTransfer == nil {
				return nil, false
			}

			return m.SONConfigurationTransfer, true
		},
	},
	apimpl.OptionalIE(IDENDCSONConfigurationTransferDL, CriticalityIgnore, func(m *DownlinkRANConfigurationTransfer) **ENDCSONConfigurationTransfer {
		return &m.ENDCSONConfigurationTransferDL
	}),
	apimpl.OptionalIE(IDIntersystemSONConfigurationTransferDL, CriticalityIgnore, func(m *DownlinkRANConfigurationTransfer) **IntersystemSONConfigurationTransfer {
		return &m.IntersystemSONConfigurationTransferDL
	}),
}

func (m *DownlinkRANConfigurationTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcDownlinkRANConfigurationTransfer, downlinkRANConfigurationTransferIEs, m)
}

func (m *DownlinkRANConfigurationTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkRANConfigurationTransfer,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseDownlinkRANConfigurationTransfer(value []byte) (*DownlinkRANConfigurationTransfer, error) {
	return parseMessageBody[DownlinkRANConfigurationTransfer](ProcDownlinkRANConfigurationTransfer, TriggeringInitiatingMessage, downlinkRANConfigurationTransferIEs, value)
}

func DecodeDownlinkRANConfigurationTransfer(value []byte) (*DownlinkRANConfigurationTransfer, error) {
	return decodeMessageBody[DownlinkRANConfigurationTransfer](ProcDownlinkRANConfigurationTransfer, downlinkRANConfigurationTransferIEs, value)
}
