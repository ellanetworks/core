// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/nas"
)

// PDU session type values (TS 24.501 §9.11.4.11, table 9.11.4.11.1).
const (
	PDUSessionTypeIPv4         PDUSessionType = 0x01
	PDUSessionTypeIPv6         PDUSessionType = 0x02
	PDUSessionTypeIPv4v6       PDUSessionType = 0x03
	PDUSessionTypeUnstructured PDUSessionType = 0x04
	PDUSessionTypeEthernet     PDUSessionType = 0x05
)

// 5GSM information element identifiers (TS 24.501 §8.3 message definitions).
// Type-1 (half-octet) IEIs are the high nibble of their octet.
const (
	iei5GSMCapability       uint8 = 0x28
	ieiMaxPacketFilters     uint8 = 0x55
	iei5GSMCause            uint8 = 0x59
	ieiPDUSessionType       uint8 = 0x90 // type 1
	ieiSSCMode              uint8 = 0xA0 // type 1
	ieiAlwaysOnRequested    uint8 = 0xB0 // type 1
	ieiSMPDUDNRequest       uint8 = 0x39
	ieiExtendedPCO          uint8 = 0x7B
	ieiIPHeaderCompression  uint8 = 0x66
	ieiDSTTPortMAC          uint8 = 0x6E
	ieiUEDSTTResidenceTime  uint8 = 0x6F
	ieiPortMgmtContainer    uint8 = 0x74
	ieiEthHeaderCompress    uint8 = 0x1F
	ieiSuggestedInterface   uint8 = 0x29
	ieiServiceLevelAA       uint8 = 0x72
	ieiRequestedMBS         uint8 = 0x70
	ieiProtoDescAccept      uint8 = 0x36 // PDU SESSION ESTABLISHMENT ACCEPT: protocol description
	ieiProtoDescCommand     uint8 = 0x38 // PDU SESSION MODIFICATION COMMAND: protocol description
	ieiPDUSessionPairID     uint8 = 0x34
	ieiIntegrityProtMaxRate uint8 = 0x13
	ieiNon3GPPDelayBudget   uint8 = 0x73
	ieiURSPEnforcement      uint8 = 0x36

	ieiSNSSAI             uint8 = 0x22
	ieiPDUAddress         uint8 = 0x29
	ieiSessionAMBR        uint8 = 0x2A
	ieiQoSFlowDescription uint8 = 0x79
	ieiDNN                uint8 = 0x25
	ieiAlwaysOnIndication uint8 = 0x80 // type 1

	ieiRQTimerValue           uint8 = 0x56 // GPRS timer (TS 24.501 §9.11.2.3)
	ieiMappedEPSBearerContext uint8 = 0x75
	ieiEAPMessageSession      uint8 = 0x78 // EAP message (same octet as the 5GMM EAP message)
	ieiAuthorizedQoSRules     uint8 = 0x7A
)

// establishmentRequestIEs is the full-octet optional-IE table of the PDU SESSION
// ESTABLISHMENT REQUEST (TS 24.501 §8.3.1, table 8.3.1.1.1). Type-1 IEs (PDU
// session type, SSC mode, always-on requested) are delimited generically by the
// walker; the table lets the walk step over the full-octet optional IEs so a
// later type-1 IE is still reached.
var establishmentRequestIEs = []nas.OptionalIE{
	{IEI: iei5GSMCapability, Format: nas.IETLV, Name: "5GSM capability"},
	{IEI: ieiMaxPacketFilters, Format: nas.IETV3, Len: 2, Name: "Maximum number of supported packet filters"},
	{IEI: ieiSMPDUDNRequest, Format: nas.IETLV, Name: "SM PDU DN request container"},
	{IEI: ieiExtendedPCO, Format: nas.IETLVE, Name: "Extended PCO"},
	{IEI: ieiIPHeaderCompression, Format: nas.IETLV, Name: "IP header compression"},
	{IEI: ieiDSTTPortMAC, Format: nas.IETLV, Name: "DS-TT Ethernet port MAC address"},
	{IEI: ieiUEDSTTResidenceTime, Format: nas.IETLV, Name: "UE-DS-TT residence time"},
	{IEI: ieiPortMgmtContainer, Format: nas.IETLVE, Name: "Port management information container"},
	{IEI: ieiEthHeaderCompress, Format: nas.IETLV, Name: "Ethernet header compression configuration"},
	{IEI: ieiSuggestedInterface, Format: nas.IETLV, Name: "Suggested interface"},
	{IEI: ieiServiceLevelAA, Format: nas.IETLVE, Name: "Service-level-AA container"},
	{IEI: ieiRequestedMBS, Format: nas.IETLVE, Name: "Requested MBS container"},
	{IEI: ieiPDUSessionPairID, Format: nas.IETLV, Name: "PDU session pair ID"},
}

// causeAndPCOIEs delimits the optional part of the release and modification
// messages that carry an optional 5GSM cause (TV, IEI 0x59) and extended PCO.
var causeAndPCOIEs = []nas.OptionalIE{
	{IEI: iei5GSMCause, Format: nas.IETV3, Len: 1, Name: "5GSM capability"},
	{IEI: ieiExtendedPCO, Format: nas.IETLVE, Name: "Extended PCO"},
}

// establishmentAcceptIEs is the full-octet optional-IE table of the PDU SESSION
// ESTABLISHMENT ACCEPT (TS 24.501 §8.3.2, table 8.3.2.1.1); the type-1 always-on
// indication is delimited generically by the walker.
var establishmentAcceptIEs = []nas.OptionalIE{
	{IEI: iei5GSMCause, Format: nas.IETV3, Len: 1, Name: "5GSM capability"},
	{IEI: ieiPDUAddress, Format: nas.IETLV, Name: "PDU address"},
	{IEI: ieiRQTimerValue, Format: nas.IETV3, Len: 1, Name: "RQ timer value"},
	{IEI: ieiSNSSAI, Format: nas.IETLV, Name: "S-NSSAI"},
	{IEI: ieiMappedEPSBearerContext, Format: nas.IETLVE, Name: "Mapped EPS bearer contexts"},
	{IEI: ieiEAPMessageSession, Format: nas.IETLVE, Name: "EAP message session"},
	{IEI: ieiQoSFlowDescription, Format: nas.IETLVE, Name: "QoS flow descriptions"},
	{IEI: ieiExtendedPCO, Format: nas.IETLVE, Name: "Extended PCO"},
	{IEI: ieiDNN, Format: nas.IETLV, Name: "DNN"},
	{IEI: ieiProtoDescAccept, Format: nas.IETLVE, Name: "Protocol description"},
}

// modificationCommandIEs is the full-octet optional-IE table of the PDU SESSION
// MODIFICATION COMMAND (TS 24.501 §8.3.9, table 8.3.9.1.1).
var modificationCommandIEs = []nas.OptionalIE{
	{IEI: iei5GSMCause, Format: nas.IETV3, Len: 1, Name: "5GSM capability"},
	{IEI: ieiRQTimerValue, Format: nas.IETV3, Len: 1, Name: "RQ timer value"}, // TV — must be tabled so the 5GS walker never mis-skips it as a TLV
	{IEI: ieiSessionAMBR, Format: nas.IETLV, Name: "Session AMBR"},
	{IEI: ieiAuthorizedQoSRules, Format: nas.IETLVE, Name: "Authorized QoS rules"},
	{IEI: ieiQoSFlowDescription, Format: nas.IETLVE, Name: "QoS flow descriptions"},
	{IEI: ieiExtendedPCO, Format: nas.IETLVE, Name: "Extended PCO"},
	{IEI: ieiProtoDescCommand, Format: nas.IETLVE, Name: "Protocol description"},
}

// SNSSAI is the S-NSSAI information element (TS 24.501 §9.11.2.8). The spec
// defines five value lengths — 1, 2, 4, 5 and 8 octets — selecting which of the
// SD and the mapped-HPLMN fields are present, so each optional part is its own
// field rather than being inferred from the remaining length.
type SNSSAI struct {
	SST            uint8
	SD             *[3]byte
	MappedHPLMNSST *uint8
	MappedHPLMNSD  *[3]byte
}

// AppendBinary encodes the S-NSSAI IE value onto b.
func (s SNSSAI) AppendBinary(b []byte) ([]byte, error) {
	switch {
	case s.SD == nil && s.MappedHPLMNSST == nil && s.MappedHPLMNSD == nil: // 1 octet
	case s.SD == nil && s.MappedHPLMNSST != nil && s.MappedHPLMNSD == nil: // 2 octets
	case s.SD != nil && s.MappedHPLMNSST == nil && s.MappedHPLMNSD == nil: // 4 octets
	case s.SD != nil && s.MappedHPLMNSST != nil && s.MappedHPLMNSD == nil: // 5 octets
	case s.SD != nil && s.MappedHPLMNSST != nil && s.MappedHPLMNSD != nil: // 8 octets
	default:
		return b, fmt.Errorf("nas/fgs: S-NSSAI: no defined length carries this combination of fields (TS 24.501 §9.11.2.8)")
	}

	b = append(b, s.SST)

	if s.SD != nil {
		b = append(b, s.SD[:]...)
	}

	if s.MappedHPLMNSST != nil {
		b = append(b, *s.MappedHPLMNSST)
	}

	if s.MappedHPLMNSD != nil {
		b = append(b, s.MappedHPLMNSD[:]...)
	}

	return b, nil
}

// MarshalBinary encodes the SNSSAI information element value.
func (s SNSSAI) MarshalBinary() ([]byte, error) { return s.AppendBinary(nil) }

// NSSAI is a list of S-NSSAIs, each length-prefixed on the wire. It is the value
// of the Allowed, Requested, and Configured NSSAI information elements
// (TS 24.501 §9.11.3.37 / §9.11.3.46).
type NSSAI []SNSSAI

// maxNSSAILen is the element's longest value: TS 24.501 §9.11.3.37 caps the
// whole element at 146 octets, two of which are its IEI and length.
const maxNSSAILen = 144

// ParseNSSAI decodes an NSSAI information element value.
func ParseNSSAI(b []byte) (NSSAI, error) {
	if len(b) > maxNSSAILen {
		return nil, fmt.Errorf("nas/fgs: NSSAI is %d octets, want at most %d", len(b), maxNSSAILen)
	}

	r := nas.NewReader(b)

	out := NSSAI{}

	for r.Remaining() > 0 {
		val, err := r.LV()
		if err != nil {
			return nil, err
		}

		s, err := ParseSNSSAI(val)
		if err != nil {
			return nil, err
		}

		out = append(out, s)
	}

	return out, nil
}

// AppendBinary encodes the NSSAI information element value onto b.
func (n NSSAI) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	for _, s := range n {
		raw, err := s.MarshalBinary()
		if err != nil {
			return b, err
		}

		w.LV(raw)
	}

	out, err := w.Result(b)
	if err != nil {
		return b, err
	}

	if len(out)-len(b) > maxNSSAILen {
		return b, fmt.Errorf("nas/fgs: NSSAI is %d octets, want at most %d", len(out)-len(b), maxNSSAILen)
	}

	return out, nil
}

// MarshalBinary encodes the NSSAI information element value.
func (n NSSAI) MarshalBinary() ([]byte, error) { return n.AppendBinary(nil) }

// ParseSNSSAI decodes an S-NSSAI IE value.
func ParseSNSSAI(v []byte) (SNSSAI, error) {
	switch len(v) {
	case 1, 2, 4, 5, 8:
	default:
		return SNSSAI{}, fmt.Errorf("nas/fgs: S-NSSAI: want 1, 2, 4, 5 or 8 octets, got %d", len(v))
	}

	s := SNSSAI{SST: v[0]}

	switch len(v) {
	case 2: // SST + mapped HPLMN SST
		sst := v[1]
		s.MappedHPLMNSST = &sst
	case 4: // SST + SD
		s.SD = sliceTo3(v[1:4])
	case 5: // SST + SD + mapped HPLMN SST
		s.SD = sliceTo3(v[1:4])
		sst := v[4]
		s.MappedHPLMNSST = &sst
	case 8: // SST + SD + mapped HPLMN SST + mapped HPLMN SD
		s.SD = sliceTo3(v[1:4])
		sst := v[4]
		s.MappedHPLMNSST = &sst
		s.MappedHPLMNSD = sliceTo3(v[5:8])
	}

	return s, nil
}

func sliceTo3(b []byte) *[3]byte {
	var a [3]byte

	copy(a[:], b)

	return &a
}

// DNN is a Data Network Name in its dotted form, encoded on the wire as RFC 1035
// labels (TS 23.003 §9A, TS 24.501 §9.11.2.1B).
type DNN string

// ParseDNN decodes a DNN information element value.
func ParseDNN(b []byte) (DNN, error) {
	name, err := nas.ParseLabelledName(b)

	return DNN(name), err
}

// AppendBinary encodes the DNN information element value onto b.
func (d DNN) AppendBinary(b []byte) ([]byte, error) { return nas.AppendLabelledName(b, string(d)) }

// MarshalBinary encodes the DNN information element value.
func (d DNN) MarshalBinary() ([]byte, error) { return d.AppendBinary(nil) }

// PDUAddress is the PDU address IE (TS 24.501 §9.11.4.10). The value is the PDU
// session type value followed by the address: a 4-octet IPv4 address, an 8-octet
// IPv6 interface identifier, or the identifier and address for IPv4v6.
type PDUAddress struct {
	SessionType PDUSessionType
	SI6LLA      bool  // bit 4: the SMF's IPv6 link-local address follows
	Spare       uint8 // bits 5-8 of the session-type octet, preserved as received
	IPv4        [4]byte
	IPv6IID     [8]byte
	Rest        []byte // octets beyond those the session type defines
}

// AppendBinary encodes the PDU address IE value onto b.
func (a PDUAddress) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	octet := uint8(a.SessionType) & 0x07
	if a.SI6LLA {
		octet |= 0x08
	}

	w.U8(octet | a.Spare&0x0F<<4)

	switch a.SessionType {
	case PDUSessionTypeIPv4:
		w.Raw(a.IPv4[:])
	case PDUSessionTypeIPv6:
		w.Raw(a.IPv6IID[:])
	case PDUSessionTypeIPv4v6:
		w.Raw(a.IPv6IID[:])
		w.Raw(a.IPv4[:])
	}

	w.Raw(a.Rest)

	return w.Result(b)
}

// MarshalBinary encodes the PDUAddress information element value.
func (a PDUAddress) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }

// IPv4Addr returns the IPv4 address the IE carries, or the zero Addr when the
// session type carries none.
func (a PDUAddress) IPv4Addr() netip.Addr {
	if a.SessionType != PDUSessionTypeIPv4 && a.SessionType != PDUSessionTypeIPv4v6 {
		return netip.Addr{}
	}

	return netip.AddrFrom4(a.IPv4)
}

// IPv6LinkLocal returns the link-local address formed from the interface
// identifier the IE carries, or the zero Addr when the session type carries
// none. The IE never carries a global prefix: the UE obtains one by SLAAC from
// the router advertisement (TS 24.501 §9.11.4.10).
func (a PDUAddress) IPv6LinkLocal() netip.Addr {
	if a.SessionType != PDUSessionTypeIPv6 && a.SessionType != PDUSessionTypeIPv4v6 {
		return netip.Addr{}
	}

	b := [16]byte{0: 0xfe, 1: 0x80}
	copy(b[8:], a.IPv6IID[:])

	return netip.AddrFrom16(b)
}

// ParsePDUAddress decodes a PDU address IE value (session type followed by the
// address).
func ParsePDUAddress(v []byte) (PDUAddress, error) {
	r := nas.NewReader(v)

	t, err := r.U8()
	if err != nil {
		return PDUAddress{}, err
	}

	a := PDUAddress{SessionType: PDUSessionType(t & 0x07), SI6LLA: t&0x08 != 0, Spare: t >> 4 & 0x0F}

	body, err := r.Bytes(r.Remaining())
	if err != nil {
		return PDUAddress{}, err
	}

	// The session type fixes the address length; anything shorter is malformed
	// rather than a zero-filled address (TS 24.501 §9.11.4.10).
	var want int

	switch a.SessionType {
	case PDUSessionTypeIPv4:
		want = 4
	case PDUSessionTypeIPv6:
		want = 8
	case PDUSessionTypeIPv4v6:
		want = 12
	}

	if len(body) < want {
		return PDUAddress{}, fmt.Errorf("nas/fgs: PDU address: session type %d wants %d octets, got %d", a.SessionType, want, len(body))
	}

	switch a.SessionType {
	case PDUSessionTypeIPv4:
		copy(a.IPv4[:], body)
	case PDUSessionTypeIPv6:
		copy(a.IPv6IID[:], body)
	case PDUSessionTypeIPv4v6:
		copy(a.IPv6IID[:], body[:8])
		copy(a.IPv4[:], body[8:12])
	}

	if len(body) > want {
		a.Rest = body[want:]
	}

	return a, nil
}
