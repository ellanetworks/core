// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// RoutingID ::= OCTET STRING, naming an LMF endpoint — TS 38.413 §9.3.3.13.
// TS 36.413 §9.2.3.33 makes its Routing-ID an INTEGER (0..255) instead, so the
// two are not the same shape on the wire.
type RoutingID []byte

func (id RoutingID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, id)
}

func (id *RoutingID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*id = RoutingID(b)

	return nil
}

// NRPPaPDU ::= OCTET STRING (unbounded), carried opaquely; the bytes are an
// NRPPa PDU (TS 38.455).
type NRPPaPDU []byte

func (p NRPPaPDU) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, p)
}

func (p *NRPPaPDU) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*p = NRPPaPDU(b)

	return nil
}

// TS 38.413 §9.2.9.1.
type DownlinkUEAssociatedNRPPaTransport struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	RoutingID   RoutingID
	NRPPaPDU    NRPPaPDU

	messageMeta
}

// The two carry identical IEs and differ only in procedure code.
func nrppaTransportIEs[M any](
	amf func(*M) *AMFUENGAPID,
	ran func(*M) *RANUENGAPID,
	routing func(*M) *RoutingID,
	pdu func(*M) *NRPPaPDU,
) []ieSpec[M] {
	return []ieSpec[M]{
		{
			id: idAMFUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, amf(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return amf(m), true },
		},
		{
			id: idRANUENGAPID, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, ran(m)) },
			encode: func(m *M) (per.Marshaler, bool) { return ran(m), true },
		},
		{
			id: idRoutingID, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, routing(m)) },
			encode: func(m *M) (per.Marshaler, bool) {
				if *routing(m) == nil {
					return nil, false
				}

				return routing(m), true
			},
		},
		{
			id: idNRPPaPDU, presence: presenceMandatory, crit: CriticalityReject,
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

var downlinkUEAssociatedNRPPaTransportIEs = nrppaTransportIEs(
	func(m *DownlinkUEAssociatedNRPPaTransport) *AMFUENGAPID { return &m.AMFUENGAPID },
	func(m *DownlinkUEAssociatedNRPPaTransport) *RANUENGAPID { return &m.RANUENGAPID },
	func(m *DownlinkUEAssociatedNRPPaTransport) *RoutingID { return &m.RoutingID },
	func(m *DownlinkUEAssociatedNRPPaTransport) *NRPPaPDU { return &m.NRPPaPDU },
)

var uplinkUEAssociatedNRPPaTransportIEs = nrppaTransportIEs(
	func(m *UplinkUEAssociatedNRPPaTransport) *AMFUENGAPID { return &m.AMFUENGAPID },
	func(m *UplinkUEAssociatedNRPPaTransport) *RANUENGAPID { return &m.RANUENGAPID },
	func(m *UplinkUEAssociatedNRPPaTransport) *RoutingID { return &m.RoutingID },
	func(m *UplinkUEAssociatedNRPPaTransport) *NRPPaPDU { return &m.NRPPaPDU },
)

func (m *DownlinkUEAssociatedNRPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcDownlinkUEAssociatedNRPPaTransport, downlinkUEAssociatedNRPPaTransportIEs, m)
}

func (m *UplinkUEAssociatedNRPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkUEAssociatedNRPPaTransport, uplinkUEAssociatedNRPPaTransportIEs, m)
}

func (m *DownlinkUEAssociatedNRPPaTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkUEAssociatedNRPPaTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseDownlinkUEAssociatedNRPPaTransport(value []byte) (*DownlinkUEAssociatedNRPPaTransport, error) {
	return parseMessageBody[DownlinkUEAssociatedNRPPaTransport](ProcDownlinkUEAssociatedNRPPaTransport, TriggeringInitiatingMessage, downlinkUEAssociatedNRPPaTransportIEs, value)
}

// TS 38.413 §9.2.9.2.
type UplinkUEAssociatedNRPPaTransport struct {
	AMFUENGAPID AMFUENGAPID
	RANUENGAPID RANUENGAPID
	RoutingID   RoutingID
	NRPPaPDU    NRPPaPDU

	messageMeta
}

func (m *UplinkUEAssociatedNRPPaTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkUEAssociatedNRPPaTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUplinkUEAssociatedNRPPaTransport(value []byte) (*UplinkUEAssociatedNRPPaTransport, error) {
	return parseMessageBody[UplinkUEAssociatedNRPPaTransport](ProcUplinkUEAssociatedNRPPaTransport, TriggeringInitiatingMessage, uplinkUEAssociatedNRPPaTransportIEs, value)
}

// The non-UE-associated pair carries only the routing id and the payload: with
// no UE context, there are no UE NGAP IDs to name.
func nonUEAssociatedNRPPaTransportIEs[M any](
	routing func(*M) *RoutingID,
	pdu func(*M) *NRPPaPDU,
) []ieSpec[M] {
	return []ieSpec[M]{
		{
			id: idRoutingID, presence: presenceMandatory, crit: CriticalityReject,
			decode: func(m *M, raw []byte, enc per.Encoding) error { return perIEDecode(raw, routing(m)) },
			encode: func(m *M) (per.Marshaler, bool) {
				if *routing(m) == nil {
					return nil, false
				}

				return routing(m), true
			},
		},
		{
			id: idNRPPaPDU, presence: presenceMandatory, crit: CriticalityReject,
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

// TS 38.413 §9.2.9.3.
type DownlinkNonUEAssociatedNRPPaTransport struct {
	RoutingID RoutingID
	NRPPaPDU  NRPPaPDU

	messageMeta
}

// TS 38.413 §9.2.9.4.
type UplinkNonUEAssociatedNRPPaTransport struct {
	RoutingID RoutingID
	NRPPaPDU  NRPPaPDU

	messageMeta
}

var downlinkNonUEAssociatedNRPPaTransportIEs = nonUEAssociatedNRPPaTransportIEs(
	func(m *DownlinkNonUEAssociatedNRPPaTransport) *RoutingID { return &m.RoutingID },
	func(m *DownlinkNonUEAssociatedNRPPaTransport) *NRPPaPDU { return &m.NRPPaPDU },
)

var uplinkNonUEAssociatedNRPPaTransportIEs = nonUEAssociatedNRPPaTransportIEs(
	func(m *UplinkNonUEAssociatedNRPPaTransport) *RoutingID { return &m.RoutingID },
	func(m *UplinkNonUEAssociatedNRPPaTransport) *NRPPaPDU { return &m.NRPPaPDU },
)

func (m *DownlinkNonUEAssociatedNRPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcDownlinkNonUEAssociatedNRPPaTransport, downlinkNonUEAssociatedNRPPaTransportIEs, m)
}

func (m *UplinkNonUEAssociatedNRPPaTransport) encodeBody(w *per.Writer, enc per.Encoding) error {
	return encodeMessageBody(w, enc, ProcUplinkNonUEAssociatedNRPPaTransport, uplinkNonUEAssociatedNRPPaTransportIEs, m)
}

func (m *DownlinkNonUEAssociatedNRPPaTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcDownlinkNonUEAssociatedNRPPaTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseDownlinkNonUEAssociatedNRPPaTransport(value []byte) (*DownlinkNonUEAssociatedNRPPaTransport, error) {
	return parseMessageBody[DownlinkNonUEAssociatedNRPPaTransport](ProcDownlinkNonUEAssociatedNRPPaTransport, TriggeringInitiatingMessage, downlinkNonUEAssociatedNRPPaTransportIEs, value)
}

func (m *UplinkNonUEAssociatedNRPPaTransport) Marshal() ([]byte, error) {
	w := per.NewWriter()

	if err := m.encodeBody(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return Marshal(&InitiatingMessage{
		ProcedureCode: ProcUplinkNonUEAssociatedNRPPaTransport,
		Criticality:   CriticalityIgnore,
		Value:         w.Bytes(),
	})
}

func ParseUplinkNonUEAssociatedNRPPaTransport(value []byte) (*UplinkNonUEAssociatedNRPPaTransport, error) {
	return parseMessageBody[UplinkNonUEAssociatedNRPPaTransport](ProcUplinkNonUEAssociatedNRPPaTransport, TriggeringInitiatingMessage, uplinkNonUEAssociatedNRPPaTransportIEs, value)
}
