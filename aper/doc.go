// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package aper implements the subset of ITU-T X.691 Packed Encoding Rules used
// by the 3GPP protocols this core speaks.
//
// Both PER variants are supported, because 3GPP does not use one throughout:
//
//   - Aligned (the default, and what [Writer]'s zero value and [NewReader]
//     give) is what S1AP (TS 36.413), NGAP, NRPPa and LPPa require.
//   - Unaligned ([NewUnalignedWriter], [NewUnalignedReader]) is what LPP
//     requires (TS 37.355 §5). It inserts no octet-alignment padding, so a
//     message encoded with the aligned variant is malformed to an LPP peer.
//
// It provides a bit-level [Writer] and [Reader] plus the whole-number, length,
// and primitive codecs that the message layers build on. Decoding never panics
// on malformed input: every read is bounds-checked and every length is
// validated against the remaining input before allocation.
//
// Section references in this package point to ITU-T X.691 (02/2021).
package aper
