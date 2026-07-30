// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// SONConfigurationTransfer is held as raw open-type bytes: it is relayed
// verbatim, and only the leading Target eNB-ID is decoded (TS 36.413 §9.2.3.26).
type SONConfigurationTransfer []byte

func (c SONConfigurationTransfer) MarshalPER(w *per.Writer, _ per.Encoding) error {
	return w.WriteOctets(c)
}

// TargetENBID decodes the destination eNB from the leading field (TS 36.413 §9.2.3.26).
func (c SONConfigurationTransfer) TargetENBID() (TargeteNBID, error) {
	r := per.NewReader(c)

	for range 2 { // extension bit + iE-Extensions presence bit
		if _, err := r.ReadBit(); err != nil {
			return TargeteNBID{}, fmt.Errorf("s1ap: SONConfigurationTransfer preamble: %w", err)
		}
	}

	var t TargeteNBID
	if err := t.UnmarshalPER(r, per.Aligned); err != nil {
		return TargeteNBID{}, err
	}

	return t, nil
}

// TS 36.413 §9.1.16.
type ENBConfigurationTransfer struct {
	SONConfigurationTransfer SONConfigurationTransfer

	messageMeta
}

var eNBConfigurationTransferIEs = []ieSpec[ENBConfigurationTransfer]{
	{
		id: idSONConfigurationTransferECT, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *ENBConfigurationTransfer, raw []byte, enc per.Encoding) error {
			m.SONConfigurationTransfer = SONConfigurationTransfer(raw)
			return nil
		},
		encode: func(m *ENBConfigurationTransfer) (per.Marshaler, bool) {
			if m.SONConfigurationTransfer == nil {
				return nil, false
			}

			return m.SONConfigurationTransfer, true
		},
	},
}

func ParseENBConfigurationTransfer(value []byte) (*ENBConfigurationTransfer, error) {
	return parseMessageBody[ENBConfigurationTransfer](ProcENBConfigurationTransfer, TriggeringInitiatingMessage, eNBConfigurationTransferIEs, value)
}

// TS 36.413 §9.1.17.
type MMEConfigurationTransfer struct {
	SONConfigurationTransfer SONConfigurationTransfer

	messageMeta
}

// IE is optional, so an empty container is a valid message.
var mMEConfigurationTransferIEs = []ieSpec[MMEConfigurationTransfer]{
	{
		id: idSONConfigurationTransferMCT, presence: PresenceOptional, crit: CriticalityIgnore,
		decode: func(m *MMEConfigurationTransfer, raw []byte, enc per.Encoding) error {
			m.SONConfigurationTransfer = SONConfigurationTransfer(raw)
			return nil
		},
		encode: func(m *MMEConfigurationTransfer) (per.Marshaler, bool) {
			if m.SONConfigurationTransfer == nil {
				return nil, false
			}

			return m.SONConfigurationTransfer, true
		},
	},
}

func (m *MMEConfigurationTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcMMEConfigurationTransfer, mMEConfigurationTransferIEs, m)
}

func (m *MMEConfigurationTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcMMEConfigurationTransfer,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}
