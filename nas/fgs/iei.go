// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

// 5GMM information element identifiers (TS 24.501 §8.2 message definitions).
const (
	ieiAUTN              uint8 = 0x20 // authentication parameter AUTN
	ieiRAND              uint8 = 0x21 // authentication parameter RAND
	ieiAuthResponseParam uint8 = 0x2D // authentication response parameter (RES*)
	ieiAuthFailureParam  uint8 = 0x30 // authentication failure parameter (AUTS)
	ieiT3502Value        uint8 = 0x16 // T3502 value (GPRS timer 2)
	ieiEAPMessage        uint8 = 0x78
	ieiPduSessionID2     uint8 = 0x12 // PDU session identity 2
	ieiAdditionalInfo    uint8 = 0x24 // additional information
	ieiCause5GMM         uint8 = 0x58 // 5GMM cause
	ieiIMEISVRequest     uint8 = 0xE0 // IMEISV request (type 1)
	ieiAdditional5GSec   uint8 = 0x36 // additional 5G security information
	ieiPDUSessionStatus  uint8 = 0x50 // PDU session status
	ieiPDUReactResult    uint8 = 0x26 // PDU session reactivation result
	ieiPDUReactErrCause  uint8 = 0x72 // PDU session reactivation result error cause
	ieiGUTI5G            uint8 = 0x77 // 5G-GUTI (5GS mobile identity)
	ieiEquivalentPlmns   uint8 = 0x4A // equivalent PLMNs
	ieiTAIList           uint8 = 0x54 // TAI list
	ieiAllowedNSSAI      uint8 = 0x15 // allowed NSSAI
	ieiNetworkFeature    uint8 = 0x21 // 5GS network feature support
	ieiT3512Value        uint8 = 0x5E // T3512 value (GPRS timer 3)
	ieiNegotiatedDRX     uint8 = 0x51 // negotiated DRX parameters
	ieiConfigUpdateInd   uint8 = 0xD0 // configuration update indication (type 1)
	ieiFullNameForNet    uint8 = 0x43 // full name for network
	ieiShortNameForNet   uint8 = 0x45 // short name for network
)
