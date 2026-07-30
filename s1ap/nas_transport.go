// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.7.1.
type InitialUEMessage struct {
	ENBUES1APID           ENBUES1APID
	NASPDU                NASPDU
	TAI                   TAI
	EUTRANCGI             EUTRANCGI
	RRCEstablishmentCause RRCEstablishmentCause
	STMSI                 *STMSI  // present when the UE re-establishes with an S-TMSI
	GUMMEI                *GUMMEI // the eNB-selected MME, present when the eNB does not run NNSF

	unmodeledIEs
}

var initialUEMessageIEs = []ieSpec[InitialUEMessage]{
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idNASPDU, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASPDU)
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.NASPDU, true },
	},
	{
		id: idTAI, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TAI)
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.TAI, true },
	},
	{
		id: idEUTRANCGI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.EUTRANCGI)
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.EUTRANCGI, true },
	},
	{
		id: idRRCEstablishmentCause, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RRCEstablishmentCause)
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.RRCEstablishmentCause, true },
	},
	{
		id: idSTMSI, presence: PresenceOptional, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			var (
				err   error
				stmsi STMSI
			)

			err = perIEDecode(raw, &stmsi)
			m.STMSI = &stmsi

			return err
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.STMSI == nil {
				return nil, false
			}

			return m.STMSI, true
		},
	},
	{
		id: idGUMMEI, presence: PresenceOptional, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			var (
				err    error
				gummei GUMMEI
			)

			err = perIEDecode(raw, &gummei)
			m.GUMMEI = &gummei

			return err
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.GUMMEI == nil {
				return nil, false
			}

			return m.GUMMEI, true
		},
	},
}

func (m *InitialUEMessage) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, initialUEMessageIEs, m)
}

func (m *InitialUEMessage) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcInitialUEMessage,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseInitialUEMessage(value []byte) (*InitialUEMessage, error) {
	return parseMessageBody[InitialUEMessage](ProcInitialUEMessage, initialUEMessageIEs, value)
}

// TS 36.413 §9.1.7.3.
type UplinkNASTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU
	EUTRANCGI   EUTRANCGI
	TAI         TAI

	unmodeledIEs
}

var uplinkNASTransportIEs = []ieSpec[UplinkNASTransport]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idNASPDU, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASPDU)
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.NASPDU, true },
	},
	{
		id: idEUTRANCGI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.EUTRANCGI)
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.EUTRANCGI, true },
	},
	{
		id: idTAI, presence: PresenceMandatory, crit: CriticalityIgnore,
		decode: func(m *UplinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.TAI)
		},
		encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.TAI, true },
	},
}

func (m *UplinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, uplinkNASTransportIEs, m)
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

func ParseUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	return parseMessageBody[UplinkNASTransport](ProcUplinkNASTransport, uplinkNASTransportIEs, value)
}

// TS 36.413 §9.1.7.2.
type DownlinkNASTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU

	unmodeledIEs
}

var downlinkNASTransportIEs = []ieSpec[DownlinkNASTransport]{
	{
		id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.MMEUES1APID)
		},
		encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.ENBUES1APID)
		},
		encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		id: idNASPDU, presence: PresenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASPDU)
		},
		encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.NASPDU, true },
	},
}

func (m *DownlinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, downlinkNASTransportIEs, m)
}

func (m *DownlinkNASTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkNASTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseDownlinkNASTransport(value []byte) (*DownlinkNASTransport, error) {
	return parseMessageBody[DownlinkNASTransport](ProcDownlinkNASTransport, downlinkNASTransportIEs, value)
}
