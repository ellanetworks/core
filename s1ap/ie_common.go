// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/s1ap/aper"

// nameMaxLen bounds ENBname / MMEname (PrintableString (SIZE(1..150,...))).
const nameMaxLen = 150

// PLMNIdentity ::= TBCD-STRING ::= OCTET STRING (SIZE(3)).
type PLMNIdentity [3]byte

func (p PLMNIdentity) encode(w *aper.Writer) error {
	return w.WriteOctetString(p[:], 3, 3, false)
}

func decodePLMNIdentity(r *aper.Reader) (PLMNIdentity, error) {
	b, err := r.ReadOctetString(3, 3, false)
	if err != nil {
		return PLMNIdentity{}, err
	}

	var p PLMNIdentity

	copy(p[:], b)

	return p, nil
}

// encodeName encodes ENBname / MMEname. Without a PER-visible alphabet
// constraint a PrintableString encodes as 8 bits per character, so it shares
// the OCTET STRING encoding with an extensible SIZE(1..150) bound (X.691).
func encodeName(w *aper.Writer, s string) error {
	return w.WriteOctetString([]byte(s), 1, nameMaxLen, true)
}

func decodeName(r *aper.Reader) (string, error) {
	b, err := r.ReadOctetString(1, nameMaxLen, true)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// uintToBits packs the low nbits of v into ceil(nbits/8) octets, most
// significant bit first, matching BIT STRING storage.
func uintToBits(v uint64, nbits int) []byte {
	out := make([]byte, (nbits+7)/8)

	for i := 0; i < nbits; i++ {
		if v&(1<<uint(nbits-1-i)) != 0 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}

	return out
}

// bitsToUint reads the first nbits of b, most significant bit first. nbits is
// clamped to the available bits so a caller cannot over-index on malformed
// input.
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
