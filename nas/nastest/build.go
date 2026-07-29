// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nastest

import (
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// BuildGMM starts a builder seeded with the plain 5GMM header (EPD 5GMM, plain
// security-header type, message type mt) — TS 24.501 §9.1.1.
func BuildGMM(mt fgs.MessageType) *Builder {
	return NewBuilder().U8(uint8(fgs.EPD5GMM)).U8(uint8(fgs.SHTPlain)).U8(uint8(mt))
}

// BuildGMMRaw starts a builder with arbitrary 5GMM header octets, for exercising
// an invalid extended protocol discriminator, security-header type, or message
// type.
func BuildGMMRaw(epd, sht, mt uint8) *Builder {
	return NewBuilder().U8(epd).U8(sht).U8(mt)
}

// BuildGSM starts a builder seeded with the plain 5GSM header (EPD 5GSM, PDU
// session identity, PTI, message type) — TS 24.007 §11.2.3.1a.
func BuildGSM(pduSessionID, pti uint8, mt fgs.GSMMessageType) *Builder {
	return NewBuilder().U8(uint8(fgs.EPD5GSM)).U8(pduSessionID).U8(pti).U8(uint8(mt))
}

// BuildGSMRaw starts a builder with arbitrary 5GSM header octets.
func BuildGSMRaw(epd, pduSessionID, pti, mt uint8) *Builder {
	return NewBuilder().U8(epd).U8(pduSessionID).U8(pti).U8(mt)
}

// BuildEMM starts a builder seeded with the plain EMM header (security-header
// type 0 in the high nibble, protocol discriminator EMM in the low nibble, then
// the message type) — TS 24.007 §11.2.3.1.1.
func BuildEMM(mt eps.MessageType) *Builder {
	return NewBuilder().U8(uint8(eps.SHTPlain)<<4 | uint8(eps.PDEMM)).U8(uint8(mt))
}

// BuildEMMRaw starts a builder with an arbitrary EMM header octet 0
// (security-header type and protocol discriminator) and message type, for
// exercising invalid values.
func BuildEMMRaw(octet0, mt uint8) *Builder {
	return NewBuilder().U8(octet0).U8(mt)
}

// BuildESM starts a builder seeded with the plain ESM header (EPS bearer
// identity in the high nibble, protocol discriminator ESM in the low nibble,
// PTI, message type) — TS 24.007 §11.2.3.1.1.
func BuildESM(ebi, pti uint8, mt eps.ESMMessageType) *Builder {
	return NewBuilder().U8(ebi<<4 | uint8(eps.PDESM)).U8(pti).U8(uint8(mt))
}

// BuildESMRaw starts a builder with arbitrary ESM header octets.
func BuildESMRaw(octet0, pti, mt uint8) *Builder {
	return NewBuilder().U8(octet0).U8(pti).U8(mt)
}
