// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// TargetRANNodeIDSON ::= SEQUENCE { globalRANNodeID, selectedTAI,
// iE-Extensions OPTIONAL } (extensible) — a field of the SON Configuration
// Transfer of TS 38.413 §9.3.3.6, with no standalone clause of its own.
//
// The plain TargetRANNodeID has the same shape but a different extension
// container.
type TargetRANNodeIDSON struct {
	_               [0]struct{} `per:"extseq"`
	GlobalRANNodeID GlobalRANNodeID
	SelectedTAI     TAI
	_               ieExtensions `per:",skip"`
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
			return TargetRANNodeIDSON{}, fmt.Errorf("ngap: SONConfigurationTransfer preamble: %w", err)
		}
	}

	var t TargetRANNodeIDSON
	if err := t.UnmarshalPER(r, per.Aligned); err != nil {
		return TargetRANNodeIDSON{}, err
	}

	return t, nil
}

// TS 38.413 §9.2.7.1.
type UplinkRANConfigurationTransfer struct {
	SONConfigurationTransfer SONConfigurationTransfer

	messageMeta
}

var uplinkRANConfigurationTransferIEs = []ieSpec[UplinkRANConfigurationTransfer]{
	{
		id: IDSONConfigurationTransferUL, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *UplinkRANConfigurationTransfer, raw []byte, enc per.Encoding) error {
			m.SONConfigurationTransfer = SONConfigurationTransfer(raw)
			return nil
		},
		encode: func(m *UplinkRANConfigurationTransfer) (per.Marshaler, bool) {
			if m.SONConfigurationTransfer == nil {
				return nil, false
			}

			return m.SONConfigurationTransfer, true
		},
	},
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

// TS 38.413 §9.2.7.2.
type DownlinkRANConfigurationTransfer struct {
	SONConfigurationTransfer SONConfigurationTransfer

	messageMeta
}

// The IE is optional, so an empty container is a valid message.
var downlinkRANConfigurationTransferIEs = []ieSpec[DownlinkRANConfigurationTransfer]{
	{
		id: IDSONConfigurationTransferDL, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *DownlinkRANConfigurationTransfer, raw []byte, enc per.Encoding) error {
			m.SONConfigurationTransfer = SONConfigurationTransfer(raw)
			return nil
		},
		encode: func(m *DownlinkRANConfigurationTransfer) (per.Marshaler, bool) {
			if m.SONConfigurationTransfer == nil {
				return nil, false
			}

			return m.SONConfigurationTransfer, true
		},
	},
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
