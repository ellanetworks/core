// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

// Enumerated is the value of an ASN.1 ENUMERATED type, held as its index within
// the enumeration.
//
// Rec. ITU-T X.691 §13 encodes that index as a constrained whole number over
// the root enumeration (§13.2), or as a normally-small non-negative number for
// an extension addition (§13.3). The 3GPP ASN.1 modules (e.g. TS 37.355) number
// their enumeration items in declaration order starting at 0, so the index is a
// small non-negative integer; int64 leaves room for the whole INTEGER range
// without unsigned-conversion surprises at call sites.
type Enumerated int64

// BitString is an ASN.1 BIT STRING (Rec. ITU-T X.691 §15).
//
// Bytes holds the bits packed most-significant-bit first: the first bit of the
// string is bit 8 (0x80) of Bytes[0]. BitLength is the number of significant
// bits; any bits in the final byte beyond BitLength are padding and are zero.
type BitString struct {
	Bytes     []byte
	BitLength int
}
