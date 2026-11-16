// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// TS 36.413 §9.1.16.
type ENBConfigurationTransfer struct {
	SONConfigurationTransfer               *SONConfigurationTransfer
	ENDCSONConfigurationTransferECT        *ENDCSONConfigurationTransfer
	IntersystemSONConfigurationTransferECT *IntersystemSONConfigurationTransfer

	messageMeta
}

var eNBConfigurationTransferIEs = []ieSpec[ENBConfigurationTransfer]{
	{
		ID: IDSONConfigurationTransferECT, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *ENBConfigurationTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.SONConfigurationTransfer)
		},
		Encode: func(m *ENBConfigurationTransfer) (per.Marshaler, bool) {
			return optionalIE(m.SONConfigurationTransfer)
		},
	},
	apimpl.OptionalIE(IDENDCSONConfigurationTransferECT, CriticalityIgnore, func(m *ENBConfigurationTransfer) **ENDCSONConfigurationTransfer {
		return &m.ENDCSONConfigurationTransferECT
	}),
	apimpl.OptionalIE(IDIntersystemSONConfigurationTransferECT, CriticalityIgnore, func(m *ENBConfigurationTransfer) **IntersystemSONConfigurationTransfer {
		return &m.IntersystemSONConfigurationTransferECT
	}),
}

func (m *ENBConfigurationTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcENBConfigurationTransfer, eNBConfigurationTransferIEs, m)
}

func (m *ENBConfigurationTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcENBConfigurationTransfer,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseENBConfigurationTransfer(value []byte) (*ENBConfigurationTransfer, error) {
	return parseMessageBody[ENBConfigurationTransfer](ProcENBConfigurationTransfer, TriggeringInitiatingMessage, eNBConfigurationTransferIEs, value)
}

func DecodeENBConfigurationTransfer(value []byte) (*ENBConfigurationTransfer, error) {
	return decodeMessageBody[ENBConfigurationTransfer](ProcENBConfigurationTransfer, eNBConfigurationTransferIEs, value)
}

// TS 36.413 §9.1.17.
type MMEConfigurationTransfer struct {
	SONConfigurationTransfer               *SONConfigurationTransfer
	ENDCSONConfigurationTransferMCT        *ENDCSONConfigurationTransfer
	IntersystemSONConfigurationTransferMCT *IntersystemSONConfigurationTransfer

	messageMeta
}

// IE is optional, so an empty container is a valid message.
var mMEConfigurationTransferIEs = []ieSpec[MMEConfigurationTransfer]{
	{
		ID: IDSONConfigurationTransferMCT, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *MMEConfigurationTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.SONConfigurationTransfer)
		},
		Encode: func(m *MMEConfigurationTransfer) (per.Marshaler, bool) {
			return optionalIE(m.SONConfigurationTransfer)
		},
	},
	apimpl.OptionalIE(IDENDCSONConfigurationTransferMCT, CriticalityIgnore, func(m *MMEConfigurationTransfer) **ENDCSONConfigurationTransfer {
		return &m.ENDCSONConfigurationTransferMCT
	}),
	apimpl.OptionalIE(IDIntersystemSONConfigurationTransferMCT, CriticalityIgnore, func(m *MMEConfigurationTransfer) **IntersystemSONConfigurationTransfer {
		return &m.IntersystemSONConfigurationTransferMCT
	}),
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

func ParseMMEConfigurationTransfer(value []byte) (*MMEConfigurationTransfer, error) {
	return parseMessageBody[MMEConfigurationTransfer](ProcMMEConfigurationTransfer, TriggeringInitiatingMessage, mMEConfigurationTransferIEs, value)
}

func DecodeMMEConfigurationTransfer(value []byte) (*MMEConfigurationTransfer, error) {
	return decodeMessageBody[MMEConfigurationTransfer](ProcMMEConfigurationTransfer, mMEConfigurationTransferIEs, value)
}
