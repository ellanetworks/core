// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"net"
	"strings"

	"github.com/ellanetworks/core/nas/common"
)

// PDU session type values (TS 24.501 §9.11.4.11, table 9.11.4.11.1).
const (
	PDUSessionTypeIPv4         uint8 = 0x01
	PDUSessionTypeIPv6         uint8 = 0x02
	PDUSessionTypeIPv4IPv6     uint8 = 0x03
	PDUSessionTypeUnstructured uint8 = 0x04
	PDUSessionTypeEthernet     uint8 = 0x05
)

// 5GSM information element identifiers (TS 24.501 §9.7 message definitions).
// Type-1 (half-octet) IEIs are the high nibble of their octet.
const (
	iei5GSMCapability      uint8 = 0x28
	ieiMaxPacketFilters    uint8 = 0x55
	iei5GSMCause           uint8 = 0x59
	ieiPDUSessionType      uint8 = 0x90 // type 1
	ieiSSCMode             uint8 = 0xA0 // type 1
	ieiAlwaysOnRequested   uint8 = 0xB0 // type 1
	ieiSMPDUDNRequest      uint8 = 0x39
	ieiExtendedPCO         uint8 = 0x7B
	ieiIPHeaderCompression uint8 = 0x66
	ieiDSTTPortMAC         uint8 = 0x6E
	ieiUEDSTTResidenceTime uint8 = 0x6F
	ieiPortMgmtContainer   uint8 = 0x74
	ieiEthHeaderCompress   uint8 = 0x1F
	ieiSuggestedInterface  uint8 = 0x29
	ieiServiceLevelAA      uint8 = 0x72
	ieiRequestedMBS        uint8 = 0x70
	ieiPDUSessionPairID    uint8 = 0x34

	ieiSNSSAI             uint8 = 0x22
	ieiPDUAddress         uint8 = 0x29
	ieiSessionAMBR        uint8 = 0x2A
	ieiQoSFlowDescription uint8 = 0x79
	ieiDNN                uint8 = 0x25
	ieiAlwaysOnIndication uint8 = 0x80 // type 1
)

// establishmentRequestIEs is the full-octet optional-IE table of the PDU SESSION
// ESTABLISHMENT REQUEST (TS 24.501 §8.3.1, table 8.3.1.1.1). Type-1 IEs (PDU
// session type, SSC mode, always-on requested) are delimited generically by the
// walker; the table lets the walk step over the full-octet optional IEs so a
// later type-1 IE is still reached.
var establishmentRequestIEs = []common.OptionalIE{
	{IEI: iei5GSMCapability, Format: common.IETLV},
	{IEI: ieiMaxPacketFilters, Format: common.IETV3, Len: 2},
	{IEI: ieiSMPDUDNRequest, Format: common.IETLV},
	{IEI: ieiExtendedPCO, Format: common.IETLVE},
	{IEI: ieiIPHeaderCompression, Format: common.IETLV},
	{IEI: ieiDSTTPortMAC, Format: common.IETLV},
	{IEI: ieiUEDSTTResidenceTime, Format: common.IETLV},
	{IEI: ieiPortMgmtContainer, Format: common.IETLVE},
	{IEI: ieiEthHeaderCompress, Format: common.IETLV},
	{IEI: ieiSuggestedInterface, Format: common.IETLV},
	{IEI: ieiServiceLevelAA, Format: common.IETLVE},
	{IEI: ieiRequestedMBS, Format: common.IETLVE},
	{IEI: ieiPDUSessionPairID, Format: common.IETLV},
}

// causeAndPCOIEs delimits the optional part of the release and modification
// messages that carry an optional 5GSM cause (TV, IEI 0x59) and extended PCO.
var causeAndPCOIEs = []common.OptionalIE{
	{IEI: iei5GSMCause, Format: common.IETV3, Len: 1},
	{IEI: ieiExtendedPCO, Format: common.IETLVE},
}

// establishmentAcceptIEs is the full-octet optional-IE table of the PDU SESSION
// ESTABLISHMENT ACCEPT (TS 24.501 §8.3.2, table 8.3.2.1.1); the type-1 always-on
// indication is delimited generically by the walker.
var establishmentAcceptIEs = []common.OptionalIE{
	{IEI: iei5GSMCause, Format: common.IETV3, Len: 1},
	{IEI: ieiPDUAddress, Format: common.IETLV},
	{IEI: 0x56, Format: common.IETV3, Len: 2}, // RQ timer value
	{IEI: ieiSNSSAI, Format: common.IETLV},
	{IEI: 0x75, Format: common.IETLVE}, // mapped EPS bearer contexts
	{IEI: 0x78, Format: common.IETLVE}, // EAP message
	{IEI: ieiQoSFlowDescription, Format: common.IETLVE},
	{IEI: ieiExtendedPCO, Format: common.IETLVE},
	{IEI: ieiDNN, Format: common.IETLV},
}

// modificationCommandIEs is the full-octet optional-IE table of the PDU SESSION
// MODIFICATION COMMAND (TS 24.501 §8.3.9, table 8.3.9.1.1).
var modificationCommandIEs = []common.OptionalIE{
	{IEI: iei5GSMCause, Format: common.IETV3, Len: 1},
	{IEI: ieiSessionAMBR, Format: common.IETLV},
	{IEI: 0x7A, Format: common.IETLVE}, // authorized QoS rules
	{IEI: ieiQoSFlowDescription, Format: common.IETLVE},
	{IEI: ieiExtendedPCO, Format: common.IETLVE},
}

// SNSSAI is the S-NSSAI IE (TS 24.501 §9.11.2.8): a mandatory SST and an
// optional 3-octet SD.
type SNSSAI struct {
	SST uint8
	SD  *[3]byte
}

func (s SNSSAI) marshalValue() []byte {
	if s.SD == nil {
		return []byte{s.SST}
	}

	return append([]byte{s.SST}, s.SD[:]...)
}

func parseSNSSAI(v []byte) *SNSSAI {
	if len(v) == 0 {
		return nil
	}

	s := &SNSSAI{SST: v[0]}

	if len(v) >= 4 {
		var sd [3]byte

		copy(sd[:], v[1:4])
		s.SD = &sd
	}

	return s
}

// labelsToDNN decodes RFC 1035 labels back to a dotted DNN string.
func labelsToDNN(v []byte) string {
	var labels []string

	for i := 0; i < len(v); {
		n := int(v[i])
		i++

		if i+n > len(v) {
			break
		}

		labels = append(labels, string(v[i:i+n]))
		i += n
	}

	return strings.Join(labels, ".")
}

// PDUAddress is the PDU address IE (TS 24.501 §9.11.4.10). The value is the PDU
// session type value followed by the address: a 4-octet IPv4 address, an 8-octet
// IPv6 interface identifier, or the identifier and address for IPv4v6.
type PDUAddress struct {
	SessionType uint8
	IPv4        [4]byte
	IPv6IID     [8]byte
}

func (a PDUAddress) marshalValue() []byte {
	var w common.Writer

	w.U8(a.SessionType & 0x07)

	switch a.SessionType {
	case PDUSessionTypeIPv4:
		w.Raw(a.IPv4[:])
	case PDUSessionTypeIPv6:
		w.Raw(a.IPv6IID[:])
	case PDUSessionTypeIPv4IPv6:
		w.Raw(a.IPv6IID[:])
		w.Raw(a.IPv4[:])
	}

	return w.Bytes()
}

// parsePDUAddress decodes a PDU address IE value (session type followed by the
// address).
func parsePDUAddress(v []byte) *PDUAddress {
	if len(v) == 0 {
		return nil
	}

	a := &PDUAddress{SessionType: v[0] & 0x07}
	body := v[1:]

	switch a.SessionType {
	case PDUSessionTypeIPv4:
		copy(a.IPv4[:], body)
	case PDUSessionTypeIPv6:
		copy(a.IPv6IID[:], body)
	case PDUSessionTypeIPv4IPv6:
		copy(a.IPv6IID[:], body)

		if len(body) >= 12 {
			copy(a.IPv4[:], body[8:12])
		}
	}

	return a
}

// NewPDUAddressIPv4 builds an IPv4 PDU address IE value from a net.IP.
func NewPDUAddressIPv4(ip net.IP) PDUAddress {
	a := PDUAddress{SessionType: PDUSessionTypeIPv4}
	copy(a.IPv4[:], ip.To4())

	return a
}

// dnnLabels encodes a DNN as RFC 1035 labels: each dot-separated label prefixed
// by its length (TS 23.003, TS 24.501 §9.11.2.1B).
func dnnLabels(dnn string) []byte {
	var w common.Writer

	for _, label := range strings.Split(dnn, ".") {
		w.U8(uint8(len(label)))
		w.Raw([]byte(label))
	}

	return w.Bytes()
}

func writeTV2(w *common.Writer, iei, value uint8) {
	w.U8(iei)
	w.U8(value)
}

func writeTLV(w *common.Writer, iei uint8, value []byte) {
	w.U8(iei)
	w.U8(uint8(len(value)))
	w.Raw(value)
}

func writeTLVE(w *common.Writer, iei uint8, value []byte) {
	w.U8(iei)
	w.U16(uint16(len(value)))
	w.Raw(value)
}
