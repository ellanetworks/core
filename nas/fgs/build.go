// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// Build starts an adversarial message builder seeded with the plain 5GMM header
// (EPD 5GMM, plain security-header-type, message type mt) — TS 24.501 §9.1.1. The
// returned common.Builder appends the mandatory fields and optional IEs (or
// deliberately malformed ones) for a compliance/adversary test.
func Build(mt MessageType) *common.Builder {
	return common.NewBuilder().U8(EPD5GMM).U8(uint8(SHTPlain)).U8(uint8(mt))
}

// BuildRaw starts a builder with arbitrary 5GMM header octets, for exercising an
// invalid extended protocol discriminator, security-header-type, or message type.
func BuildRaw(epd, sht, mt uint8) *common.Builder {
	return common.NewBuilder().U8(epd).U8(sht).U8(mt)
}

// BuildGSM starts a builder seeded with the plain 5GSM header (EPD 5GSM, PDU session
// identity, PTI, message type) — TS 24.007 §11.2.3.1a.
func BuildGSM(pduSessionID, pti uint8, mt GSMMessageType) *common.Builder {
	return common.NewBuilder().U8(EPD5GSM).U8(pduSessionID).U8(pti).U8(uint8(mt))
}

// BuildGSMRaw starts a builder with arbitrary 5GSM header octets.
func BuildGSMRaw(epd, pduSessionID, pti, mt uint8) *common.Builder {
	return common.NewBuilder().U8(epd).U8(pduSessionID).U8(pti).U8(mt)
}
