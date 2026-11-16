// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// The eNB Status Transfer Transparent Container as raw open-type bytes,
// relayed verbatim: the MME does not interpret the PDCP-SN/HFN COUNTs.
type StatusTransferContainer []byte

func (c StatusTransferContainer) MarshalPER(w *per.Writer, _ per.Encoding) error {
	return w.WriteOctets(c)
}

// TS 36.413 §9.1.5.13.
type ENBStatusTransfer struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Container   StatusTransferContainer

	messageMeta
}

var eNBStatusTransferIEs = []ieSpec[ENBStatusTransfer]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ENBStatusTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ENBStatusTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDENBStatusTransferTransparentContainer, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *ENBStatusTransfer, r *per.Reader, enc per.Encoding) error {
			m.Container = StatusTransferContainer(restOctets(r))
			return nil
		},
		Encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) {
			if m.Container == nil {
				return nil, false
			}

			return m.Container, true
		},
	},
}

func (m *ENBStatusTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcENBStatusTransfer, eNBStatusTransferIEs, m)
}

func (m *ENBStatusTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcENBStatusTransfer,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseENBStatusTransfer(value []byte) (*ENBStatusTransfer, error) {
	return parseMessageBody[ENBStatusTransfer](ProcENBStatusTransfer, TriggeringInitiatingMessage, eNBStatusTransferIEs, value)
}

func DecodeENBStatusTransfer(value []byte) (*ENBStatusTransfer, error) {
	return decodeMessageBody[ENBStatusTransfer](ProcENBStatusTransfer, eNBStatusTransferIEs, value)
}

// TS 36.413 §9.1.5.14.
type MMEStatusTransfer struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Container   StatusTransferContainer

	messageMeta
}

var mMEStatusTransferIEs = []ieSpec[MMEStatusTransfer]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *MMEStatusTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *MMEStatusTransfer, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDENBStatusTransferTransparentContainer, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *MMEStatusTransfer, r *per.Reader, enc per.Encoding) error {
			m.Container = StatusTransferContainer(restOctets(r))
			return nil
		},
		Encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) {
			if m.Container == nil {
				return nil, false
			}

			return m.Container, true
		},
	},
}

func (m *MMEStatusTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcMMEStatusTransfer, mMEStatusTransferIEs, m)
}

func (m *MMEStatusTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcMMEStatusTransfer,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseMMEStatusTransfer(value []byte) (*MMEStatusTransfer, error) {
	return parseMessageBody[MMEStatusTransfer](ProcMMEStatusTransfer, TriggeringInitiatingMessage, mMEStatusTransferIEs, value)
}

func DecodeMMEStatusTransfer(value []byte) (*MMEStatusTransfer, error) {
	return decodeMessageBody[MMEStatusTransfer](ProcMMEStatusTransfer, mMEStatusTransferIEs, value)
}
