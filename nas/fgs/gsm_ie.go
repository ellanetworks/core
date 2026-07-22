// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

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
