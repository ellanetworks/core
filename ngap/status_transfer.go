// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// The RAN Status Transfer Transparent Container as raw open-type bytes,
// relayed verbatim: the AMF does not interpret the PDCP-SN/HFN COUNTs.
type StatusTransferContainer []byte

func (c StatusTransferContainer) MarshalPER(w *per.Writer, _ per.Encoding) error {
	return w.WriteOctets(c)
}

// TS 38.413 §9.2.3.9. TS 36.413 §9.1.5.13 names the same message eNB Status
// Transfer and carries the same three IEs with the same criticality.
type UplinkRANStatusTransfer struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	Container   StatusTransferContainer

	messageMeta
}

var uplinkRANStatusTransferIEs = []ieSpec[UplinkRANStatusTransfer]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkRANStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *UplinkRANStatusTransfer) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkRANStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *UplinkRANStatusTransfer) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idRANStatusTransferTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkRANStatusTransfer, raw []byte, enc per.Encoding) error {
			m.Container = StatusTransferContainer(raw)

			return nil
		},
		encode: func(m *UplinkRANStatusTransfer) (per.Marshaler, bool) {
			if m.Container == nil {
				return nil, false
			}

			return m.Container, true
		},
	},
}

func (m *UplinkRANStatusTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkRANStatusTransfer, uplinkRANStatusTransferIEs, m)
}

func (m *UplinkRANStatusTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkRANStatusTransfer,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseUplinkRANStatusTransfer(value []byte) (*UplinkRANStatusTransfer, error) {
	return parseMessageBody[UplinkRANStatusTransfer](ProcUplinkRANStatusTransfer, TriggeringInitiatingMessage, uplinkRANStatusTransferIEs, value)
}

// TS 38.413 §9.2.3.8. TS 36.413 §9.1.5.14 names the same message MME Status
// Transfer and carries the same three IEs with the same criticality.
type DownlinkRANStatusTransfer struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	Container   StatusTransferContainer

	messageMeta
}

var downlinkRANStatusTransferIEs = []ieSpec[DownlinkRANStatusTransfer]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkRANStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *DownlinkRANStatusTransfer) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkRANStatusTransfer, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *DownlinkRANStatusTransfer) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idRANStatusTransferTransparentContainer, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkRANStatusTransfer, raw []byte, enc per.Encoding) error {
			m.Container = StatusTransferContainer(raw)

			return nil
		},
		encode: func(m *DownlinkRANStatusTransfer) (per.Marshaler, bool) {
			if m.Container == nil {
				return nil, false
			}

			return m.Container, true
		},
	},
}

func (m *DownlinkRANStatusTransfer) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcDownlinkRANStatusTransfer, downlinkRANStatusTransferIEs, m)
}

func (m *DownlinkRANStatusTransfer) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkRANStatusTransfer,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

func ParseDownlinkRANStatusTransfer(value []byte) (*DownlinkRANStatusTransfer, error) {
	return parseMessageBody[DownlinkRANStatusTransfer](ProcDownlinkRANStatusTransfer, TriggeringInitiatingMessage, downlinkRANStatusTransferIEs, value)
}
