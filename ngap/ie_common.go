// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per"

// nameMaxLen bounds AMFName / RANNodeName (PrintableString (SIZE(1..150,...))).
const nameMaxLen = 150

// PLMNIdentity ::= OCTET STRING (SIZE(3)).
type PLMNIdentity [3]byte

// Name is an AMFName / RANNodeName: PrintableString (SIZE(1..150,...)). In the
// ALIGNED variant PrintableString encodes 8 bits per character (X.691 §30.5.2).
type Name string

func (n Name) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeKnownMultiplierString(w, enc, per.CharPrintableString, 1, nameMaxLen, true, true, true, string(n))
}

func (n *Name) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	s, err := per.DecodeKnownMultiplierString(r, enc, per.CharPrintableString, 1, nameMaxLen, true, true, true)
	if err != nil {
		return err
	}

	*n = Name(s)

	return nil
}

// TransportLayerAddress ::= BIT STRING (SIZE(1..160, ...)). Holds the address
// octets (IPv4 = 4, IPv6 = 16).
type TransportLayerAddress []byte

func (a TransportLayerAddress) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeBitString(w, enc, 1, 160, true, true, true, a, len(a)*8)
}

func (a *TransportLayerAddress) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, nbits, err := per.DecodeBitString(r, enc, 1, 160, true, true, true)
	if err != nil {
		return err
	}

	*a = TransportLayerAddress(b[:(nbits+7)/8])

	return nil
}

// Most significant bit first, matching BIT STRING storage.
func uintToBits(v uint64, nbits int) []byte {
	out := make([]byte, (nbits+7)/8)

	for i := 0; i < nbits; i++ {
		if v&(1<<uint(nbits-1-i)) != 0 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}

	return out
}

// nbits is clamped to the available bits so a caller cannot over-index on
// malformed input.
func bitsToUint(b []byte, nbits int) uint64 {
	if nbits > len(b)*8 {
		nbits = len(b) * 8
	}

	var v uint64

	for i := 0; i < nbits; i++ {
		if b[i/8]&(1<<uint(7-i%8)) != 0 {
			v |= 1 << uint(nbits-1-i)
		}
	}

	return v
}
