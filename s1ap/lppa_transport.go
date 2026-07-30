// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// RoutingID ::= INTEGER (0..255) (TS 36.413). Identifies the E-SMLC endpoint the
// carried LPPa-PDU is routed to or from.
type RoutingID uint8

func (id RoutingID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(id))
}

func (id *RoutingID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
	if err != nil {
		return err
	}

	*id = RoutingID(v)

	return nil
}

// LPPaPDU ::= OCTET STRING (unbounded). The S1AP layer carries an LPPa PDU
// opaquely; the bytes are decoded by the LPPa codec (TS 36.455), not here.
type LPPaPDU []byte

func (p LPPaPDU) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, p)
}

func (p *LPPaPDU) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*p = LPPaPDU(b)

	return nil
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

// Marshal encodes the message as a complete S1AP-PDU.
func (m *DownlinkUEAssociatedLPPaTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkUEAssociatedLPPaTransport,
		Criticality:   CriticalityIgnore,
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

// Marshal encodes the message as a complete S1AP-PDU.
func (m *UplinkUEAssociatedLPPaTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkUEAssociatedLPPaTransport,
		Criticality:   CriticalityIgnore,
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
// transport messages.
func encodeLPPaTransportBody(w *per.Writer, enc per.Encoding, mme MMEUES1APID, enb ENBUES1APID, routing RoutingID, pdu LPPaPDU, unknown []rawIE) error {
	w.WriteBit(false)

	fields := []ieField{
		{id: idMMEUES1APID, crit: CriticalityReject, val: &mme},
		{id: idENBUES1APID, crit: CriticalityReject, val: &enb},
		{id: idRoutingID, crit: CriticalityReject, val: &routing},
		{id: idLPPaPDU, crit: CriticalityReject, val: &pdu},
	}

	for _, e := range unknown {
		fields = append(fields, e.field())
	}

	return encodeIEContainer(w, enc, fields)
}

func (m *DownlinkUEAssociatedLPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeLPPaTransportBody(w, enc, m.MMEUES1APID, m.ENBUES1APID, m.RoutingID, m.LPPaPDU, m.unknownIEs)
}

func (m *UplinkUEAssociatedLPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeLPPaTransportBody(w, enc, m.MMEUES1APID, m.ENBUES1APID, m.RoutingID, m.LPPaPDU, m.unknownIEs)
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

	r := per.NewReader(value)
	enc := per.Aligned

	extPresent, err := r.ReadBit()
	if err != nil {
		return f, fmt.Errorf("s1ap: LPPa transport preamble: %w", err)
	}

	fields, err := decodeIEContainer(r, enc)
	if err != nil {
		return f, err
	}

	if extPresent {
		if err := skipSequenceExtensionsPER(r, enc, false, true); err != nil {
			return f, err
		}
	}

	var seenMME, seenENB, seenRouting, seenPDU bool

	for _, ie := range fields {
		switch ie.id {
		case idMMEUES1APID:
			err = perIEDecode(ie.value, &f.mme)
			seenMME = true
		case idENBUES1APID:
			err = perIEDecode(ie.value, &f.enb)
			seenENB = true
		case idRoutingID:
			err = perIEDecode(ie.value, &f.routing)
			seenRouting = true
		case idLPPaPDU:
			err = perIEDecode(ie.value, &f.pdu)
			seenPDU = true
		default:
			f.unknown = append(f.unknown, ie)
		}

		if err != nil {
			return f, fmt.Errorf("s1ap: LPPa transport IE %d: %w", ie.id, err)
		}
	}

	if err := requireIEs(ProcDownlinkUEAssociatedLPPaTransport,
		ieCheck{idMMEUES1APID, CriticalityReject, seenMME},
		ieCheck{idENBUES1APID, CriticalityReject, seenENB},
		ieCheck{idRoutingID, CriticalityReject, seenRouting},
		ieCheck{idLPPaPDU, CriticalityReject, seenPDU},
	); err != nil {
		return f, err
	}

	return f, nil
}
