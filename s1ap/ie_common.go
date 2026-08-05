// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// encodeExtensibleInt encodes an INTEGER whose constraint carries an extension
// marker but defines no extension additions: X.691 §13.1 spends one bit, then
// the root value follows. A value outside the root is refused rather than sent
// as an extension, because 3GPP has defined no such addition to send.
func encodeExtensibleInt(w *per.Writer, enc per.Encoding, lb, ub, n int64) error {
	if n < lb || n > ub {
		return fmt.Errorf("value %d outside the extension root %d..%d", n, lb, ub)
	}

	w.WriteBit(false)

	return per.EncodeInteger(w, enc, per.Bounds{LB: lb, HasLB: true, UB: ub, HasUB: true}, n)
}

// decodeExtensibleInt consumes an extension value before reporting it, so the
// reader is left positioned after the field: X.691 §13.1 sends an out-of-root
// value as an unconstrained integer (§13.2.4-13.2.6), which carries its own
// length determinant and is therefore skippable without knowing the addition.
// Criticality then decides what to do with the enclosing IE (§10.3.1 case 6).
func decodeExtensibleInt(r *per.Reader, enc per.Encoding, lb, ub int64, name string) (int64, error) {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: lb, HasLB: true, UB: ub, HasUB: true, Extensible: true})
	if err != nil {
		return 0, err
	}

	if v < lb || v > ub {
		return 0, fmt.Errorf("%w: %s extension value %d", errNotComprehended, name, v)
	}

	return v, nil
}

// nameMaxLen bounds ENBname / MMEname (PrintableString (SIZE(1..150,...))).
const nameMaxLen = 150

// PLMNIdentity ::= TBCD-STRING ::= OCTET STRING (SIZE(3)).
type PLMNIdentity [3]byte

// Name is an ENBname / MMEname: PrintableString (SIZE(1..150,...)). In the
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
