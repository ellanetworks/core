// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import "testing"

// FuzzReaderNoPanic asserts the decode primitives never panic on arbitrary
// input. The decoded values are irrelevant; only the no-panic invariant and
// clean error returns matter.
func FuzzReaderNoPanic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x80, 0x01})
	f.Add([]byte{0x40, 0x01, 0x00})
	f.Add([]byte{0xbf, 0xff, 0xde, 0xad, 0xbe, 0xef})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(data)
		_, _ = r.ReadConstrainedInt(0, 4294967295)
		_, _ = r.ReadConstrainedInt(0, 255)
		_, _ = r.ReadSemiConstrainedInt(0)
		_, _ = r.ReadUnconstrainedInt()
		_, _ = r.ReadNormallySmall()
		_, _ = r.ReadLength()
		_, _ = r.ReadConstrainedLength(0, 1000)
		n, _ := r.ReadConstrainedInt(0, 10)
		_, _ = r.ReadOctets(int(n))
		_, _ = r.ReadOctetString(0, Unbounded, false)
		_, _ = r.ReadOctetString(3, 3, false)
		_, _ = r.ReadOctetString(1, 4, true)
		_, _, _ = r.ReadBitString(20, 20, false)
		_, _, _ = r.ReadBitString(1, 160, true)
		_, _ = r.ReadOpenType()
		_, _, _ = r.ReadEnum(3, true)
		_, _, _ = r.ReadChoiceIndex(5, false)
		_, _ = r.ReadNSLength()
		_, _, _ = r.ReadSequencePreamble(true, 4)
		_ = r.SkipExtensionAdditions()
		_ = r.Align()
		_, _ = r.ReadBits(13)
	})
}
