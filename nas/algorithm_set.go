// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "strconv"

// AlgorithmSet is one octet of a UE's advertised security algorithms, as the
// network capability and security capability elements code them: algorithm n
// occupies bit 8-n, so algorithm 0 is the most significant bit (TS 24.501
// §9.11.3.54, TS 24.301 §9.9.3.34, §9.9.3.36).
//
// Which family the identities name — 5G-EA, EIA, UEA, GEA — is fixed by the
// field the set sits in, and some families leave the bit of algorithm 0 spare or
// give a bit to something else. The capability types state those exceptions on
// their own accessors; this type only reads and writes bits.
type AlgorithmSet uint8

// maxAlgorithmIdentity is the highest identity an octet can advertise.
const maxAlgorithmIdentity = 7

// Algorithms is the set advertising each of the given algorithm identities.
// An identity above 7 has no bit in the octet and is ignored.
func Algorithms(identities ...uint8) AlgorithmSet {
	var s AlgorithmSet

	for _, n := range identities {
		s = s.With(n)
	}

	return s
}

// With is s extended with algorithm n, or s unchanged if n has no bit.
func (s AlgorithmSet) With(n uint8) AlgorithmSet {
	if n > maxAlgorithmIdentity {
		return s
	}

	return s | 1<<(maxAlgorithmIdentity-n)
}

// Supports reports whether the set advertises algorithm n.
func (s AlgorithmSet) Supports(n uint8) bool {
	return n <= maxAlgorithmIdentity && s&(1<<(maxAlgorithmIdentity-n)) != 0
}

// Identities lists the algorithms the set advertises, in ascending order.
func (s AlgorithmSet) Identities() []uint8 {
	var out []uint8

	for n := uint8(0); n <= maxAlgorithmIdentity; n++ {
		if s.Supports(n) {
			out = append(out, n)
		}
	}

	return out
}

func (s AlgorithmSet) String() string {
	if s == 0 {
		return "none"
	}

	out := make([]byte, 0, 16)

	for _, n := range s.Identities() {
		if len(out) > 0 {
			out = append(out, ',')
		}

		out = strconv.AppendUint(out, uint64(n), 10)
	}

	return string(out)
}
