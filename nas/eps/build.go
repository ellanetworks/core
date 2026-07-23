// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// Build starts an adversarial message builder seeded with the plain EMM header
// (security-header-type 0 in the high nibble, protocol discriminator EMM in the
// low nibble, then the message type) — TS 24.007 §11.2.3.1.1. The returned
// common.Builder appends the mandatory fields and optional IEs (or deliberately
// malformed ones) for a compliance/adversary test.
func Build(mt MessageType) *common.Builder {
	return common.NewBuilder().U8(uint8(SHTPlain)<<4 | PDEMM).U8(uint8(mt))
}

// BuildRaw starts a builder with an arbitrary EMM header octet 0 (security-header-type
// and protocol discriminator) and message type, for exercising invalid values.
func BuildRaw(octet0, mt uint8) *common.Builder {
	return common.NewBuilder().U8(octet0).U8(mt)
}

// BuildESM starts a builder seeded with the plain ESM header (EPS bearer identity in
// the high nibble, protocol discriminator ESM in the low nibble, PTI, message type) —
// TS 24.007 §11.2.3.1.1.
func BuildESM(ebi, pti uint8, mt ESMMessageType) *common.Builder {
	return common.NewBuilder().U8(ebi<<4 | PDESM).U8(pti).U8(uint8(mt))
}

// BuildESMRaw starts a builder with arbitrary ESM header octets.
func BuildESMRaw(octet0, pti, mt uint8) *common.Builder {
	return common.NewBuilder().U8(octet0).U8(pti).U8(mt)
}
