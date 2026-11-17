// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"strconv"

	"github.com/ellanetworks/core/per"
)

type BitString struct {
	octets string
	bits   int
}

func BitStringFrom(b []byte, nbits int) BitString {
	nbits = max(0, min(nbits, len(b)*8))
	n := (nbits + 7) / 8
	octets := make([]byte, n)
	copy(octets, b[:n])

	if rem := nbits % 8; rem != 0 {
		octets[n-1] &= byte(0xff << (8 - rem))
	}

	return BitString{octets: string(octets), bits: nbits}
}

func BitStringFromUint(v uint64, nbits int) BitString {
	return BitStringFrom(uintToBits(v, nbits), nbits)
}

func (b BitString) Bits() int { return b.bits }

func (b BitString) Bytes() []byte { return []byte(b.octets) }

func (b BitString) Uint64() uint64 {
	if b.bits > 64 {
		return bitsToUint([]byte(b.octets[:8]), 64)
	}

	return bitsToUint([]byte(b.octets), b.bits)
}

func (b BitString) String() string {
	return hex.EncodeToString([]byte(b.octets)) + "/" + strconv.Itoa(b.bits)
}

func (b BitString) encode(w *per.Writer, enc per.Encoding, lb, ub int64) error {
	return per.EncodeBitString(w, enc, lb, ub, true, true, true, []byte(b.octets), b.bits)
}

func decodeBitString(r *per.Reader, enc per.Encoding, lb, ub int64) (BitString, error) {
	data, nbits, err := per.DecodeBitString(r, enc, lb, ub, true, true, true)
	if err != nil {
		return BitString{}, err
	}

	return BitStringFrom(data, nbits), nil
}

func (b BitString) Equal(y BitString) bool { return b == y }
