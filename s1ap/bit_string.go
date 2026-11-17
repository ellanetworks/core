// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

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

func (b BitString) Bits() int { return b.bits }

func (b BitString) Bytes() []byte { return []byte(b.octets) }

func (b BitString) String() string {
	return hex.EncodeToString([]byte(b.octets)) + "/" + strconv.Itoa(b.bits)
}

func (b BitString) Equal(y BitString) bool { return b == y }

func (b BitString) encode(w *per.Writer, enc per.Encoding, lb, ub int64, ext bool) error {
	return per.EncodeBitString(w, enc, lb, ub, true, true, ext, []byte(b.octets), b.bits)
}

func decodeBitString(r *per.Reader, enc per.Encoding, lb, ub int64, ext bool) (BitString, error) {
	data, nbits, err := per.DecodeBitString(r, enc, lb, ub, true, true, ext)
	if err != nil {
		return BitString{}, err
	}

	return BitStringFrom(data, nbits), nil
}

const uint64RangeOctets = 8

func encodeUint64Range(w *per.Writer, enc per.Encoding, v uint64) error {
	if enc == per.Unaligned {
		w.WriteBits(v, 64)

		return nil
	}

	octets := 1
	for x := v >> 8; x != 0; x >>= 8 {
		octets++
	}

	return per.EncodeLength(w, enc, 1, uint64RangeOctets, true, int64(octets), func(int64) error {
		w.AlignToByte()

		for i := octets - 1; i >= 0; i-- {
			w.WriteBits(v>>(8*i)&0xff, 8)
		}

		return nil
	})
}

func decodeUint64Range(r *per.Reader, enc per.Encoding) (uint64, error) {
	if enc == per.Unaligned {
		return r.ReadBits(64)
	}

	var v uint64

	err := per.DecodeLength(r, enc, 1, uint64RangeOctets, true, func(count int64) error {
		r.AlignToByte()

		for range count {
			b, err := r.ReadBits(8)
			if err != nil {
				return err
			}

			v = v<<8 | b
		}

		return nil
	})

	return v, err
}
