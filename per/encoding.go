// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package per implements the ASN.1 Packed Encoding Rules (Rec. ITU-T X.691),
// both the Aligned and the Unaligned variant.
//
// The package is reflection-free: types are encoded through the [Marshaler]
// and [Unmarshaler] interfaces, normally implemented by per/cmd/pergen.
package per

// Encoding selects a PER variant.
type Encoding uint8

const (
	// Aligned is the BASIC-PER Aligned variant: fields are padded to octet
	// boundaries where the rules require it. It is the default Encoding.
	Aligned Encoding = iota
	// Unaligned is the BASIC-PER Unaligned variant: bits pack densely.
	Unaligned
)

func (e Encoding) String() string {
	switch e {
	case Aligned:
		return "aligned"
	case Unaligned:
		return "unaligned"
	default:
		return "unknown"
	}
}
