// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

// 5GMM information element identifiers this decoder renders (TS 24.501 §8.2). An
// IEI's meaning is message-scoped (TS 24.007 §11.2.4); each is named for its meaning.
const (
	ieiIMEISV      uint8 = 0x77 // SECURITY MODE COMPLETE IMEISV (same octet as the additional GUTI)
	ieiExtendedPCO uint8 = 0x7b // extended protocol configuration options (same octet as the payload container)
)
