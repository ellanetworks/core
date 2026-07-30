// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// SONConfigurationTransfer holds the SON Configuration Transfer IE
// (TS 36.413 §9.2.3.26) as raw open-type bytes: the MME relays it verbatim and
// decodes only the leading Target eNB-ID to route it.
type SONConfigurationTransfer []byte

// MarshalPER writes the container's octets as the IE's open-type content. The
// MME relays the transfer verbatim and decodes only the target eNB-ID.
func (c SONConfigurationTransfer) MarshalPER(w *per.Writer, _ per.Encoding) error {
	return w.WriteOctets(c)
}

// TargetENBID decodes the leading Target eNB-ID, which names the destination eNB
// (TS 36.413 §9.2.3.26). The remaining fields (source eNB-ID, SON Information) are
// relayed as opaque bytes.
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

// ENBConfigurationTransfer is the ENB CONFIGURATION TRANSFER message
// (TS 36.413 §8.15), sent by an eNB to convey SON configuration for another eNB.
// SONConfigurationTransfer is nil when the optional IE is absent. Only the base
// variant is modelled; EN-DC and inter-system SON transfers round-trip as unknown IEs.
type ENBConfigurationTransfer struct {
	SONConfigurationTransfer SONConfigurationTransfer

	unmodeledIEs
}

// ParseENBConfigurationTransfer decodes the message from an initiatingMessage
// open-type payload.
// eNBConfigurationTransferIEs is the ENBConfigurationTransfer IE table (TS 36.413 §9.1.16). Every
// IE is optional, so an empty container is a valid message.
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
	return parseMessageBody[ENBConfigurationTransfer](ProcENBConfigurationTransfer, eNBConfigurationTransferIEs, value)
}

// MMEConfigurationTransfer is the MME CONFIGURATION TRANSFER message
// (TS 36.413 §8.16), sent by the MME to relay a SON Configuration Transfer IE to
// the target eNB.
type MMEConfigurationTransfer struct {
	SONConfigurationTransfer SONConfigurationTransfer

	unmodeledIEs
}

// mMEConfigurationTransferIEs is the MMEConfigurationTransfer IE table (TS 36.413 §9.1.17). Every
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
	return encodeMessageBody(w, enc, mMEConfigurationTransferIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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
