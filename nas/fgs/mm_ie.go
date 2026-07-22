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
)
