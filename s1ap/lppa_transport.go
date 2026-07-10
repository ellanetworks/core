// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// RoutingID ::= INTEGER (0..255) (TS 36.413). Identifies the E-SMLC endpoint the
// carried LPPa-PDU is routed to or from.
type RoutingID uint8

func (id RoutingID) encode(w *aper.Writer) error {
	return w.WriteConstrainedInt(int64(id), 0, 255)
}

func decodeRoutingID(r *aper.Reader) (RoutingID, error) {
	v, err := r.ReadConstrainedInt(0, 255)
	return RoutingID(v), err
}

// LPPaPDU ::= OCTET STRING (unbounded). The S1AP layer carries an LPPa PDU
// opaquely; the bytes are decoded by the LPPa codec (TS 36.455), not here.
type LPPaPDU []byte

func (p LPPaPDU) encode(w *aper.Writer) error {
	return w.WriteOctetString(p, 0, aper.Unbounded, false)
}

func decodeLPPaPDU(r *aper.Reader) (LPPaPDU, error) {
	b, err := r.ReadOctetString(0, aper.Unbounded, false)
	return LPPaPDU(b), err
}

// DownlinkUEAssociatedLPPaTransport is the DOWNLINK UE ASSOCIATED LPPA TRANSPORT
// message (TS 36.413), sent by the MME to relay an LPPa PDU to the eNB.
type DownlinkUEAssociatedLPPaTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	RoutingID   RoutingID
	LPPaPDU     LPPaPDU

	unmodeledIEs
}

func (m *DownlinkUEAssociatedLPPaTransport) encodeBody(w *aper.Writer) error {
	return encodeLPPaTransportBody(w, m.MMEUES1APID, m.ENBUES1APID, m.RoutingID, m.LPPaPDU, m.unknownIEs)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *DownlinkUEAssociatedLPPaTransport) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkUEAssociatedLPPaTransport,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseDownlinkUEAssociatedLPPaTransport decodes the message from the open-type
// payload of an initiatingMessage.
func ParseDownlinkUEAssociatedLPPaTransport(value []byte) (*DownlinkUEAssociatedLPPaTransport, error) {
	f, err := decodeLPPaTransportBody(value)
	if err != nil {
		return nil, err
	}

	return &DownlinkUEAssociatedLPPaTransport{
		MMEUES1APID:  f.mme,
		ENBUES1APID:  f.enb,
		RoutingID:    f.routing,
		LPPaPDU:      f.pdu,
		unmodeledIEs: unmodeledIEs{unknownIEs: f.unknown},
	}, nil
}

// UplinkUEAssociatedLPPaTransport is the UPLINK UE ASSOCIATED LPPA TRANSPORT
// message (TS 36.413), sent by the eNB to relay an LPPa PDU to the MME.
type UplinkUEAssociatedLPPaTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	RoutingID   RoutingID
	LPPaPDU     LPPaPDU

	unmodeledIEs
}

func (m *UplinkUEAssociatedLPPaTransport) encodeBody(w *aper.Writer) error {
	return encodeLPPaTransportBody(w, m.MMEUES1APID, m.ENBUES1APID, m.RoutingID, m.LPPaPDU, m.unknownIEs)
}

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UplinkUEAssociatedLPPaTransport) Marshal() ([]byte, error) {
	var w aper.Writer

	if err := m.encodeBody(&w); err != nil {
		return nil, err
	}

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkUEAssociatedLPPaTransport,
		Criticality:   CriticalityReject,
		Value:         w.Bytes(),
	})
}

// ParseUplinkUEAssociatedLPPaTransport decodes the message from the open-type
// payload of an initiatingMessage.
func ParseUplinkUEAssociatedLPPaTransport(value []byte) (*UplinkUEAssociatedLPPaTransport, error) {
	f, err := decodeLPPaTransportBody(value)
	if err != nil {
		return nil, err
	}

	return &UplinkUEAssociatedLPPaTransport{
		MMEUES1APID:  f.mme,
		ENBUES1APID:  f.enb,
		RoutingID:    f.routing,
		LPPaPDU:      f.pdu,
		unmodeledIEs: unmodeledIEs{unknownIEs: f.unknown},
	}, nil
}

// encodeLPPaTransportBody writes the shared body of the two UE-associated LPPa
// transport messages: MME-UE-S1AP-ID, eNB-UE-S1AP-ID, Routing-ID, LPPa-PDU, all
// mandatory with reject criticality (TS 36.413 §9.1).
func encodeLPPaTransportBody(w *aper.Writer, mme MMEUES1APID, enb ENBUES1APID, routing RoutingID, pdu LPPaPDU, unknown []rawIE) error {
	w.WriteSequencePreamble(true, false, nil)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, enc: mme.encode},
		{id: idENBUES1APID, crit: CriticalityReject, enc: enb.encode},
		{id: idRoutingID, crit: CriticalityReject, enc: routing.encode},
		{id: idLPPaPDU, crit: CriticalityReject, enc: pdu.encode},
	}

	for _, e := range unknown {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, fields)
}

// lppaTransportFields holds the decoded body of a UE-associated LPPa transport
// message.
type lppaTransportFields struct {
	mme     MMEUES1APID
	enb     ENBUES1APID
	routing RoutingID
	pdu     LPPaPDU
	unknown []rawIE
}

func decodeLPPaTransportBody(value []byte) (lppaTransportFields, error) {
	var f lppaTransportFields

	r := aper.NewReader(value)

	extPresent, _, err := r.ReadSequencePreamble(true, 0)
	if err != nil {
		return f, fmt.Errorf("s1ap: LPPa transport preamble: %w", err)
	}

	fields, err := decodeIEContainer(r)
	if err != nil {
		return f, err
	}

	if extPresent {
		if err := r.SkipExtensionAdditions(); err != nil {
			return f, err
		}
	}

	var seenMME, seenENB, seenRouting, seenPDU bool

	for _, ie := range fields {
		sub := aper.NewReader(ie.value)

		switch ie.id {
		case idMMEUES1APID:
			f.mme, err = decodeMMEUES1APID(sub)
			seenMME = true
		case idENBUES1APID:
			f.enb, err = decodeENBUES1APID(sub)
			seenENB = true
		case idRoutingID:
			f.routing, err = decodeRoutingID(sub)
			seenRouting = true
		case idLPPaPDU:
			f.pdu, err = decodeLPPaPDU(sub)
			seenPDU = true
		default:
			f.unknown = append(f.unknown, ie)
		}

		if err != nil {
			return f, fmt.Errorf("s1ap: LPPa transport IE %d: %w", ie.id, err)
		}
	}

	if !seenMME || !seenENB || !seenRouting || !seenPDU {
		return f, fmt.Errorf("s1ap: LPPa transport missing mandatory IE")
	}

	return f, nil
}
