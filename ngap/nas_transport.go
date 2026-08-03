// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.5.3.
//
// S1AP carries the location as two mandatory IEs, E-UTRAN CGI and TAI; NGAP
// carries the single User Location Information CHOICE (§9.3.1.16).
type UplinkNASTransport struct {
	AMFUENGAPID             AMFUENGAPID
	RANUENGAPID             RANUENGAPID
	NASPDU                  NASPDU
	UserLocationInformation *UserLocationInformation

	messageMeta
}

func (m *UplinkNASTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkNASTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

var uplinkNASTransportIEs = []ieSpec[UplinkNASTransport]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idNASPDU, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASPDU)
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			var v UserLocationInformation

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UserLocationInformation = &v

			return nil
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) {
			if m.UserLocationInformation == nil {
				return nil, false
			}

			return m.UserLocationInformation, true
		},
	},
}

func (m *UplinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkNASTransport, uplinkNASTransportIEs, m)
}

func ParseUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	return parseMessageBody[UplinkNASTransport](ProcUplinkNASTransport, TriggeringInitiatingMessage, uplinkNASTransportIEs, value)
}
