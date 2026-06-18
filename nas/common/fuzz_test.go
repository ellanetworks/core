// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import "testing"

// FuzzReaderNoPanic asserts the Reader never panics on arbitrary input — the
// invariant the whole codec relies on for malformed-packet safety.
func FuzzReaderNoPanic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x07, 0x41, 0x01})
	f.Add([]byte{0xff, 0xff, 0x00, 0x01})
	// the real Attach Request NAS-PDU (testdata/captures/attach_request_nas.hex)
	f.Add([]byte{0x3b, 0x17, 0xdf, 0x67, 0x5a, 0xa8, 0x05, 0x07, 0x41, 0x02, 0x0b, 0xf6})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(data)
		for r.Remaining() > 0 {
			if _, err := r.U8(); err != nil {
				break
			}
		}

		_, _ = NewReader(data).LV()
		_, _ = NewReader(data).LVE()

		r2 := NewReader(data)
		_, _ = r2.U16()
		_, _ = r2.Bytes(len(data) + 10) // deliberate over-read
		_, _ = r2.PeekU8()
	})
}
