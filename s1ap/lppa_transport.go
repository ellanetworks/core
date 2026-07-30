// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
)

// RoutingID ::= INTEGER (0..255), naming an E-SMLC endpoint (TS 36.413).
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

// LPPaPDU ::= OCTET STRING (unbounded), carried opaquely; the bytes are an
// LPPa PDU (TS 36.455).
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

// TS 36.413 §9.1.19.1.
type DownlinkUEAssociatedLPPaTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	RoutingID   RoutingID
	LPPaPDU     LPPaPDU

	unmodeledIEs
}

// The two UE-associated LPPa transport messages carry identical IEs and differ
// only in procedure code (TS 36.413 §9.1.19.1, §9.1.19.2).
func lppaTransportIEs[M any](
	mme func(*M) *MMEUES1APID,
	enb func(*M) *ENBUES1APID,
	routing func(*M) *RoutingID,
	pdu func(*M) *LPPaPDU,
) []ieSpec[M] {
	return []ieSpec[M]{
		{
			id: idMMEUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, mme(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return mme(m), true },
		},
		{
			id: idENBUES1APID, presence: PresenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, enb(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return enb(m), true },
		},
		{
			id: idRoutingID, presence: PresenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, routing(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return routing(m), true },
		},
		{
			id: idLPPaPDU, presence: PresenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, pdu(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return pdu(m), true },
		},
	}
}

var downlinkUEAssociatedLPPaTransportIEs = lppaTransportIEs(
	func(m *DownlinkUEAssociatedLPPaTransport) *MMEUES1APID { return &m.MMEUES1APID },
	func(m *DownlinkUEAssociatedLPPaTransport) *ENBUES1APID { return &m.ENBUES1APID },
	func(m *DownlinkUEAssociatedLPPaTransport) *RoutingID { return &m.RoutingID },
	func(m *DownlinkUEAssociatedLPPaTransport) *LPPaPDU { return &m.LPPaPDU },
)

var uplinkUEAssociatedLPPaTransportIEs = lppaTransportIEs(
	func(m *UplinkUEAssociatedLPPaTransport) *MMEUES1APID { return &m.MMEUES1APID },
	func(m *UplinkUEAssociatedLPPaTransport) *ENBUES1APID { return &m.ENBUES1APID },
	func(m *UplinkUEAssociatedLPPaTransport) *RoutingID { return &m.RoutingID },
	func(m *UplinkUEAssociatedLPPaTransport) *LPPaPDU { return &m.LPPaPDU },
)

func (m *DownlinkUEAssociatedLPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, downlinkUEAssociatedLPPaTransportIEs, m)
}

func (m *UplinkUEAssociatedLPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, uplinkUEAssociatedLPPaTransportIEs, m)
}

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

func ParseDownlinkUEAssociatedLPPaTransport(value []byte) (*DownlinkUEAssociatedLPPaTransport, error) {
	return parseMessageBody[DownlinkUEAssociatedLPPaTransport](ProcDownlinkUEAssociatedLPPaTransport, downlinkUEAssociatedLPPaTransportIEs, value)
}

// TS 36.413 §9.1.19.2.
type UplinkUEAssociatedLPPaTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	RoutingID   RoutingID
	LPPaPDU     LPPaPDU

	unmodeledIEs
}

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

func ParseUplinkUEAssociatedLPPaTransport(value []byte) (*UplinkUEAssociatedLPPaTransport, error) {
	return parseMessageBody[UplinkUEAssociatedLPPaTransport](ProcUplinkUEAssociatedLPPaTransport, uplinkUEAssociatedLPPaTransportIEs, value)
}
