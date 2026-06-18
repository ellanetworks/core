// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package aper implements the subset of ITU-T X.691 Aligned Packed Encoding
// Rules (APER) used by 3GPP S1AP (TS 36.413).
//
// It provides a bit-level [Writer] and [Reader] plus the whole-number, length,
// and primitive codecs that the generated message layer builds on. Decoding
// never panics on malformed input: every read is bounds-checked and every
// length is validated against the remaining input before allocation.
//
// Section references in this package point to ITU-T X.691 (02/2021).
package aper
