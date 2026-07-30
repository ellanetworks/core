// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// InitialUEMessage is the INITIAL UE MESSAGE (TS 36.413), sent by the
// eNB to deliver a UE's first NAS message. Unmodeled IEs are preserved.
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

// initialUEMessageIEs is the InitialUEMessage IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseInitialUEMessage decodes an InitialUEMessage from the open-type payload
// of an initiatingMessage.
func ParseInitialUEMessage(value []byte) (*InitialUEMessage, error) {
	return parseMessageBody[InitialUEMessage](ProcInitialUEMessage, initialUEMessageIEs, value)
}

// UplinkNASTransport is the UPLINK NAS TRANSPORT message (TS 36.413),
// sent by the eNB to relay a UE's NAS message on an established UE context.
type UplinkNASTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU
	EUTRANCGI   EUTRANCGI
	TAI         TAI

	unmodeledIEs
}

// uplinkNASTransportIEs is the UplinkNASTransport IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseUplinkNASTransport decodes an UplinkNASTransport from the open-type
// payload of an initiatingMessage.
func ParseUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	return parseMessageBody[UplinkNASTransport](ProcUplinkNASTransport, uplinkNASTransportIEs, value)
}

// DownlinkNASTransport is the DOWNLINK NAS TRANSPORT message (TS 36.413),
// sent by the MME to relay a NAS message to the UE.
type DownlinkNASTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	NASPDU      NASPDU

	unmodeledIEs
}

// downlinkNASTransportIEs is the DownlinkNASTransport IE table (TS 36.413).
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

// Marshal encodes the message as a complete S1AP-PDU.
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

// ParseDownlinkNASTransport decodes a DownlinkNASTransport from the open-type
// payload of an initiatingMessage.
func ParseDownlinkNASTransport(value []byte) (*DownlinkNASTransport, error) {
	return parseMessageBody[DownlinkNASTransport](ProcDownlinkNASTransport, downlinkNASTransportIEs, value)
}
