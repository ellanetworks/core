// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/aper"
)

// SONConfigurationTransfer holds the SON Configuration Transfer IE
// (TS 36.413 §9.2.3.26) as raw open-type bytes: the MME relays it verbatim and
// decodes only the leading Target eNB-ID to route it.
type SONConfigurationTransfer []byte

func (c SONConfigurationTransfer) field(id ProtocolIEID) ieField {
	return ieField{id: id, crit: CriticalityIgnore, enc: func(w *aper.Writer) error {
		w.WriteOctets(c)
		return nil
	}}
}

// TargetENBID decodes the leading Target eNB-ID, which names the destination eNB
// (TS 36.413 §9.2.3.26). The remaining fields (source eNB-ID, SON Information) are
// relayed as opaque bytes.
func (c SONConfigurationTransfer) TargetENBID() (TargeteNBID, error) {
	r := aper.NewReader(c)

	if _, _, err := r.ReadSequencePreamble(true, 1); err != nil {
		return TargeteNBID{}, fmt.Errorf("s1ap: SONConfigurationTransfer preamble: %w", err)
	}

	return decodeTargeteNBID(r)
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
func ParseENBConfigurationTransfer(value []byte) (*ENBConfigurationTransfer, error) {
	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return nil, fmt.Errorf("s1ap: ENBConfigurationTransfer preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return nil, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return nil, err
		}
	}

	m := &ENBConfigurationTransfer{}

	for _, f := range fields {
		switch f.id {
		case idSONConfigurationTransferECT:
			m.SONConfigurationTransfer = SONConfigurationTransfer(f.value)
		default:
			m.unknownIEs = append(m.unknownIEs, f)
		}
	}

	return m, nil
}

// MMEConfigurationTransfer is the MME CONFIGURATION TRANSFER message
// (TS 36.413 §8.16), sent by the MME to relay a SON Configuration Transfer IE to
// the target eNB.
type MMEConfigurationTransfer struct {
	SONConfigurationTransfer SONConfigurationTransfer

	unmodeledIEs
}

func (m *MMEConfigurationTransfer) encodeBody(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, nil)

	var fields []ieField

	if m.SONConfigurationTransfer != nil {
		fields = append(fields, m.SONConfigurationTransfer.field(idSONConfigurationTransferMCT))
	}

	for _, e := range m.unknownIEs {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *MMEConfigurationTransfer) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcMMEConfigurationTransfer,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}
