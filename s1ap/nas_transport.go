// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// TS 36.413 §9.1.7.1.
type InitialUEMessage struct {
	ENBUES1APID                   ENBUES1APID
	NASPDU                        NASPDU
	TAI                           TAI
	EUTRANCGI                     *EUTRANCGI
	RRCEstablishmentCause         *RRCEstablishmentCause
	STMSI                         *STMSI  // present when the UE re-establishes with an S-TMSI
	GUMMEI                        *GUMMEI // the eNB-selected MME, present when the eNB does not run NNSF
	GWTransportLayerAddress       *TransportLayerAddress
	TunnelInformationForBBF       *TunnelInformation
	SIPTOLGWTransportLayerAddress *TransportLayerAddress
	CoarseUELocationRequested     *CoarseUELocationRequested

	messageMeta
}

var initialUEMessageIEs = []ieSpec[InitialUEMessage]{
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDNASPDU, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASPDU)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	{
		ID: IDTAI, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.TAI)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return &m.TAI, true },
	},
	{
		ID: IDEUTRANCGI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var v EUTRANCGI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.EUTRANCGI = &v

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.EUTRANCGI == nil {
				return nil, false
			}

			return m.EUTRANCGI, true
		},
	},
	{
		ID: IDRRCEstablishmentCause, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var v RRCEstablishmentCause

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.RRCEstablishmentCause = &v

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.RRCEstablishmentCause == nil {
				return nil, false
			}

			return m.RRCEstablishmentCause, true
		},
	},
	{
		ID: IDSTMSI, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var (
				err   error
				stmsi STMSI
			)

			err = decodeIE(r, &stmsi)
			if err != nil {
				return err
			}

			m.STMSI = &stmsi

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.STMSI == nil {
				return nil, false
			}

			return m.STMSI, true
		},
	},
	{
		ID: IDGUMMEI, Presence: presenceOptional, Criticality: CriticalityReject,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			var (
				err    error
				gummei GUMMEI
			)

			err = decodeIE(r, &gummei)
			if err != nil {
				return err
			}

			m.GUMMEI = &gummei

			return nil
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) {
			if m.GUMMEI == nil {
				return nil, false
			}

			return m.GUMMEI, true
		},
	},
	{
		ID: IDGWTransportLayerAddress, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.GWTransportLayerAddress)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return optionalIE(m.GWTransportLayerAddress) },
	},
	{
		ID: IDTunnelInformationForBBF, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.TunnelInformationForBBF)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return optionalIE(m.TunnelInformationForBBF) },
	},
	{
		ID: IDSIPTOLGWTransportLayerAddress, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.SIPTOLGWTransportLayerAddress)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return optionalIE(m.SIPTOLGWTransportLayerAddress) },
	},
	{
		ID: IDCoarseUELocationRequested, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *InitialUEMessage, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.CoarseUELocationRequested)
		},
		Encode: func(m *InitialUEMessage) (per.Marshaler, bool) { return optionalIE(m.CoarseUELocationRequested) },
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

func DecodeInitialUEMessage(value []byte) (*InitialUEMessage, error) {
	return decodeMessageBody[InitialUEMessage](ProcInitialUEMessage, initialUEMessageIEs, value)
}

// TS 36.413 §9.1.7.3.
type UplinkNASTransport struct {
	MMEUES1APID                   MMEUES1APID
	ENBUES1APID                   ENBUES1APID
	NASPDU                        NASPDU
	EUTRANCGI                     *EUTRANCGI
	TAI                           *TAI
	GWTransportLayerAddress       *TransportLayerAddress
	SIPTOLGWTransportLayerAddress *TransportLayerAddress

	messageMeta
}

var uplinkNASTransportIEs = []ieSpec[UplinkNASTransport]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		// NAS-PDU is an unconstrained OCTET STRING, so a zero-length value is a
		// legal thing to carry and nil means absent, not empty: the guard below
		// must test the slice's nil-ness, not its length, or a relayed empty
		// container would be dropped.
		ID: IDNASPDU, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASPDU)
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	{
		ID: IDEUTRANCGI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			var v EUTRANCGI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.EUTRANCGI = &v

			return nil
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) {
			if m.EUTRANCGI == nil {
				return nil, false
			}

			return m.EUTRANCGI, true
		},
	},
	{
		ID: IDTAI, Presence: presenceMandatory, Criticality: CriticalityIgnore,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			var v TAI

			if err := decodeIE(r, &v); err != nil {
				return err
			}

			m.TAI = &v

			return nil
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) {
			if m.TAI == nil {
				return nil, false
			}

			return m.TAI, true
		},
	},
	{
		ID: IDGWTransportLayerAddress, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.GWTransportLayerAddress)
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return optionalIE(m.GWTransportLayerAddress) },
	},
	{
		ID: IDSIPTOLGWTransportLayerAddress, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *UplinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.SIPTOLGWTransportLayerAddress)
		},
		Encode: func(m *UplinkNASTransport) (per.Marshaler, bool) { return optionalIE(m.SIPTOLGWTransportLayerAddress) },
	},
}

func (m *UplinkNASTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkNASTransport, uplinkNASTransportIEs, m)
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
	return parseMessageBody[UplinkNASTransport](ProcUplinkNASTransport, TriggeringInitiatingMessage, uplinkNASTransportIEs, value)
}

func DecodeUplinkNASTransport(value []byte) (*UplinkNASTransport, error) {
	return decodeMessageBody[UplinkNASTransport](ProcUplinkNASTransport, uplinkNASTransportIEs, value)
}

// TS 36.413 §9.1.7.2.
type DownlinkNASTransport struct {
	MMEUES1APID      MMEUES1APID
	ENBUES1APID      ENBUES1APID
	NASPDU           NASPDU
	CoarseUELocation *CoarseUELocation

	messageMeta
}

var downlinkNASTransportIEs = []ieSpec[DownlinkNASTransport]{
	{
		ID: IDMMEUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *DownlinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.MMEUES1APID)
		},
		Encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.MMEUES1APID, true },
	},
	{
		ID: IDENBUES1APID, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *DownlinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.ENBUES1APID)
		},
		Encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return &m.ENBUES1APID, true },
	},
	{
		ID: IDNASPDU, Presence: presenceMandatory, Criticality: CriticalityReject,
		Decode: func(m *DownlinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeIE(r, &m.NASPDU)
		},
		Encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) {
			if m.NASPDU == nil {
				return nil, false
			}

			return &m.NASPDU, true
		},
	},
	{
		ID: IDCoarseUELocation, Presence: presenceOptional, Criticality: CriticalityIgnore,
		Decode: func(m *DownlinkNASTransport, r *per.Reader, enc per.Encoding) error {
			return decodeOptionalIE(r, &m.CoarseUELocation)
		},
		Encode: func(m *DownlinkNASTransport) (per.Marshaler, bool) { return optionalIE(m.CoarseUELocation) },
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

func DecodeDownlinkNASTransport(value []byte) (*DownlinkNASTransport, error) {
	return decodeMessageBody[DownlinkNASTransport](ProcDownlinkNASTransport, downlinkNASTransportIEs, value)
}
