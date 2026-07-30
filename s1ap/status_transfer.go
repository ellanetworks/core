// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// StatusTransferContainer holds the eNB Status Transfer Transparent Container
// (TS 36.413) as its raw open-type value bytes. The MME does not
// interpret the PDCP-SN/HFN COUNT values it carries; it relays the container
// verbatim from ENB STATUS TRANSFER into MME STATUS TRANSFER.
type StatusTransferContainer []byte

// MarshalPER writes the container's octets as the IE's open-type content. The
// MME does not interpret them; it relays the container verbatim.
func (c StatusTransferContainer) MarshalPER(w *per.Writer, _ per.Encoding) error {
	return w.WriteOctets(c)
}

// ENBStatusTransfer is the ENB STATUS TRANSFER message (TS 36.413 in
// the eNB Status Transfer procedure), sent by the source eNB to convey PDCP-SN
// and HFN status to the target eNB via the MME.
type ENBStatusTransfer struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Container   StatusTransferContainer

	unmodeledIEs
}

// eNBStatusTransferIEs is the ENBStatusTransfer IE table (TS 36.413 §9.1.5.13/§9.1.5.14).
var eNBStatusTransferIEs = []ieSpec[ENBStatusTransfer]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ENBStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ENBStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idENBStatusTransferTransparentContainer, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *ENBStatusTransfer, raw []byte, enc per.Encoding) error {
			m.Container = StatusTransferContainer(raw)
			return nil
		},
		encode: func(m *ENBStatusTransfer) (per.Marshaler, bool) { return m.Container, true },
	},
}

func (m *ENBStatusTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, eNBStatusTransferIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseENBStatusTransfer decodes the message from an initiatingMessage open-type
// payload.
func ParseENBStatusTransfer(value []byte) (*ENBStatusTransfer, error) {
	return parseMessageBody[ENBStatusTransfer](ProcENBStatusTransfer, eNBStatusTransferIEs, value)
}

// MMEStatusTransfer is the MME STATUS TRANSFER message (TS 36.413 in
// the MME Status Transfer procedure), sent by the MME to relay the source eNB's
// status container to the target eNB.
type MMEStatusTransfer struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	Container   StatusTransferContainer

	unmodeledIEs
}

// mMEStatusTransferIEs is the MMEStatusTransfer IE table (TS 36.413 §9.1.5.13/§9.1.5.14).
var mMEStatusTransferIEs = []ieSpec[MMEStatusTransfer]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *MMEStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *MMEStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idENBStatusTransferTransparentContainer, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *MMEStatusTransfer, raw []byte, enc per.Encoding) error {
			m.Container = StatusTransferContainer(raw)
			return nil
		},
		encode: func(m *MMEStatusTransfer) (per.Marshaler, bool) { return m.Container, true },
	},
}

func (m *MMEStatusTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, mMEStatusTransferIEs, m)
}

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseMMEStatusTransfer decodes the message from an initiatingMessage open-type
// payload.
func ParseMMEStatusTransfer(value []byte) (*MMEStatusTransfer, error) {
	return parseMessageBody[MMEStatusTransfer](ProcMMEStatusTransfer, mMEStatusTransferIEs, value)
}
