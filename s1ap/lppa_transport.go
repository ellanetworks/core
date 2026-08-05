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

	messageMeta
}

// The two carry identical IEs and differ only in procedure code.
func lppaTransportIEs[M any](
	mme func(*M) *MMEUES1APID,
	enb func(*M) *ENBUES1APID,
	routing func(*M) *RoutingID,
	pdu func(*M) *LPPaPDU,
) []ieSpec[M] {
	return []ieSpec[M]{
		{
			id: idMMEUES1APID, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, mme(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return mme(m), true },
		},
		{
			id: idENBUES1APID, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, enb(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return enb(m), true },
		},
		{
			id: idRoutingID, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, routing(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return routing(m), true },
		},
		{
			id: idLPPaPDU, presence: presenceMandatory, crit: CriticalityReject,
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
	return encodeMessageBody(w, enc, ProcDownlinkUEAssociatedLPPaTransport, downlinkUEAssociatedLPPaTransportIEs, m)
}

func (m *UplinkUEAssociatedLPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkUEAssociatedLPPaTransport, uplinkUEAssociatedLPPaTransportIEs, m)
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
	return parseMessageBody[DownlinkUEAssociatedLPPaTransport](ProcDownlinkUEAssociatedLPPaTransport, TriggeringInitiatingMessage, downlinkUEAssociatedLPPaTransportIEs, value)
}

// TS 36.413 §9.1.19.2.
type UplinkUEAssociatedLPPaTransport struct {
	MMEUES1APID MMEUES1APID
	ENBUES1APID ENBUES1APID
	RoutingID   RoutingID
	LPPaPDU     LPPaPDU

	messageMeta
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
	return parseMessageBody[UplinkUEAssociatedLPPaTransport](ProcUplinkUEAssociatedLPPaTransport, TriggeringInitiatingMessage, uplinkUEAssociatedLPPaTransportIEs, value)
}

// The non-UE-associated pair carries only the routing id and the payload: with
// no UE context, there are no UE S1AP IDs to name.
func nonUEAssociatedLPPaTransportIEs[M any](
	routing func(*M) *RoutingID,
	pdu func(*M) *LPPaPDU,
) []ieSpec[M] {
	return []ieSpec[M]{
		{
			id: idRoutingID, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, routing(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return routing(m), true },
		},
		{
			id: idLPPaPDU, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, pdu(m)) },
			encode: func(m *M) (per.Marshaler, bool) {
				if *pdu(m) == nil {
					return nil, false
				}

				return pdu(m), true
			},
		},
	}
}

// TS 36.413 §9.1.19.3.
type DownlinkNonUEAssociatedLPPaTransport struct {
	RoutingID RoutingID
	LPPaPDU   LPPaPDU

	messageMeta
}

// TS 36.413 §9.1.19.4.
type UplinkNonUEAssociatedLPPaTransport struct {
	RoutingID RoutingID
	LPPaPDU   LPPaPDU

	messageMeta
}

var downlinkNonUEAssociatedLPPaTransportIEs = nonUEAssociatedLPPaTransportIEs(
	func(m *DownlinkNonUEAssociatedLPPaTransport) *RoutingID { return &m.RoutingID },
	func(m *DownlinkNonUEAssociatedLPPaTransport) *LPPaPDU { return &m.LPPaPDU },
)

var uplinkNonUEAssociatedLPPaTransportIEs = nonUEAssociatedLPPaTransportIEs(
	func(m *UplinkNonUEAssociatedLPPaTransport) *RoutingID { return &m.RoutingID },
	func(m *UplinkNonUEAssociatedLPPaTransport) *LPPaPDU { return &m.LPPaPDU },
)

func (m *DownlinkNonUEAssociatedLPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcDownlinkNonUEAssociatedLPPaTransport, downlinkNonUEAssociatedLPPaTransportIEs, m)
}

func (m *UplinkNonUEAssociatedLPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkNonUEAssociatedLPPaTransport, uplinkNonUEAssociatedLPPaTransportIEs, m)
}

func (m *DownlinkNonUEAssociatedLPPaTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkNonUEAssociatedLPPaTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseDownlinkNonUEAssociatedLPPaTransport(value []byte) (*DownlinkNonUEAssociatedLPPaTransport, error) {
	return parseMessageBody[DownlinkNonUEAssociatedLPPaTransport](ProcDownlinkNonUEAssociatedLPPaTransport, TriggeringInitiatingMessage, downlinkNonUEAssociatedLPPaTransportIEs, value)
}

func (m *UplinkNonUEAssociatedLPPaTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkNonUEAssociatedLPPaTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUplinkNonUEAssociatedLPPaTransport(value []byte) (*UplinkNonUEAssociatedLPPaTransport, error) {
	return parseMessageBody[UplinkNonUEAssociatedLPPaTransport](ProcUplinkNonUEAssociatedLPPaTransport, TriggeringInitiatingMessage, uplinkNonUEAssociatedLPPaTransportIEs, value)
}
