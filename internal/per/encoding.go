// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package per implements the Packed Encoding Rules (PER) for ASN.1.
//
// PER encodes ASN.1 abstract values into a concrete octet (and sub-octet)
// representation. Two variants are supported:
//
//   - Aligned PER (the default), and
//   - Unaligned PER (opt-in via per.Unaligned).
//
// The package is reflection-free. Types that wish to be (un)marshalled must
// implement [Marshaler] and [Unmarshaler], typically by running the pergen
// code generator (see per/cmd/pergen). The top-level [Marshal] and [Unmarshal]
// helpers dispatch through these interfaces with no use of the reflect package.
package per

// Encoding selects a PER variant.
type Encoding uint8

const (
	// Aligned is the BASIC-PER Aligned variant: fields are padded to octet
	// boundaries where the rules require it. It is the default Encoding.
	Aligned Encoding = iota
	// Unaligned is the BASIC-PER Unaligned variant: no padding is inserted
	// between fields; bits pack densely.
	Unaligned
)

// String returns the variant name.
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
