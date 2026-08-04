// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// TS 38.413 §9.2.5.1.
//
// S1AP carries the location as TAI (mandatory reject) plus E-UTRAN CGI
// (mandatory ignore); NGAP merges both into User Location Information, which is
// mandatory reject and therefore a value type here — where UPLINK NAS TRANSPORT
// marks the same IE ignore and it is nil-able.
type InitialUEMessage struct {
	RANUENGAPID             RANUENGAPID
	NASPDU                  NASPDU
	UserLocationInformation UserLocationInformation
	RRCEstablishmentCause   *RRCEstablishmentCause
	// §8.6.2.2: the NG-RAN node includes the 5G-S-TMSI whenever the UE supplied
	// it over the radio interface, and the AMF Set ID only on a message it has
	// rerouted, which the AMF acts on per TS 23.502. The AMF Set ID is not the
	// counterpart of S1AP's GUMMEI, which marks an eNB that does not run NNSF.
	FiveGSTMSI       *FiveGSTMSI
	AMFSetID         *AMFSetID
	UEContextRequest *UEContextRequest

	messageMeta
}

var initialUEMessageIEs = []ieSpec[InitialUEMessage]{
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idNASPDU, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASPDU)
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	{
		id: idUserLocationInformation, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.UserLocationInformation)
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.UserLocationInformation, true },
	},
	{
		id: idRRCEstablishmentCause, presence: presenceMandatory, crit: CriticalityIgnore,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			var v RRCEstablishmentCause

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.RRCEstablishmentCause = &v

			return nil
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.RRCEstablishmentCause == nil {
				return nil, false
			}

			return m.RRCEstablishmentCause, true
		},
	},
	{
		id: idFiveGSTMSI, presence: presenceOptional, crit: CriticalityReject,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			var v FiveGSTMSI

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.FiveGSTMSI = &v

			return nil
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.FiveGSTMSI == nil {
				return nil, false
			}

			return m.FiveGSTMSI, true
		},
	},
	{
		id: idAMFSetID, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			var v AMFSetID

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.AMFSetID = &v

			return nil
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.AMFSetID == nil {
				return nil, false
			}

			return m.AMFSetID, true
		},
	},
	{
		id: idUEContextRequest, presence: presenceOptional, crit: CriticalityIgnore,
		decode: func(m *InitialUEMessage, raw []byte, enc per.Encoding) error {
			var v UEContextRequest

			if err := perIEDecode(raw, &v); err != nil {
				return err
			}

			m.UEContextRequest = &v

			return nil
		},
		encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.UEContextRequest == nil {
				return nil, false
			}

			return m.UEContextRequest, true
		},
	},
}

func (m *InitialUEMessage) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcInitialUEMessage, initialUEMessageIEs, m)
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
	return parseMessageBody[InitialUEMessage](ProcInitialUEMessage, TriggeringInitiatingMessage, initialUEMessageIEs, value)
}

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

func (m *UplinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkNASTransport, uplinkNASTransportIEs, m)
}

func ParseUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	return parseMessageBody[UplinkNASTransport](ProcUplinkNASTransport, TriggeringInitiatingMessage, uplinkNASTransportIEs, value)
}

// TS 38.413 §9.2.5.2.
//
// NGAP places NAS-PDU after the optional Old AMF and RAN Paging Priority IEs
// where S1AP puts it third, but neither optional IE is modeled, so the modeled
// table order matches.
type DownlinkNASTransport struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	NASPDU      NASPDU

	messageMeta
}

var downlinkNASTransportIEs = []ieSpec[DownlinkNASTransport]{
	{
		id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.AMFUENGAPID)
		},
		encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.AMFUENGAPID, true },
	},
	{
		id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.RANUENGAPID)
		},
		encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.RANUENGAPID, true },
	},
	{
		id: idNASPDU, presence: presenceMandatory, crit: CriticalityReject,
		decode: func(m *DownlinkNASTransport, raw []byte, enc per.Encoding) error {
			return perIEDecode(raw, &m.NASPDU)
		},
		encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
}

func (m *DownlinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcDownlinkNASTransport, downlinkNASTransportIEs, m)
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
	return parseMessageBody[DownlinkNASTransport](ProcDownlinkNASTransport, TriggeringInitiatingMessage, downlinkNASTransportIEs, value)
}
