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
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ENBStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ENBStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idENBStatusTransferTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *ENBStatusTransfer, raw []byte, enc per.Encoding) error {
			m.Container = StatusTransferContainer(raw)
			return nil
		},
		encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) { return m.Container, true },
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
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseENBStatusTransfer(value []byte) (*ENBStatusTransfer, error) {
	return parseMessageBody[ENBStatusTransfer](ProcENBStatusTransfer, TriggeringInitiatingMessage, eNBStatusTransferIEs, value)
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
		id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *MMEStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *MMEStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idENBStatusTransferTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *MMEStatusTransfer, raw []byte, enc per.Encoding) error {
			m.Container = StatusTransferContainer(raw)
			return nil
		},
		encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) { return m.Container, true },
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
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseMMEStatusTransfer(value []byte) (*MMEStatusTransfer, error) {
	return parseMessageBody[MMEStatusTransfer](ProcMMEStatusTransfer, TriggeringInitiatingMessage, mMEStatusTransferIEs, value)
}
