// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// writeOctetAligned writes p as an "octet-aligned bit-field" (§11.1.4): in the
// ALIGNED variant the writer is padded to an octet boundary first, then whole
// octets are written; in the UNALIGNED variant the bits are packed densely with
// no padding (the field is just len(p)*8 bits).
func writeOctetAligned(w *Writer, enc Encoding, p []byte) {
	if enc == Aligned {
		w.AlignToByte()
		_ = w.WriteOctets(p)

		return
	}

	w.WriteBitString(p, len(p)*8)
}

// readOctetAligned reads n octets as an octet-aligned bit-field (§11.1.4): in
// the ALIGNED variant the reader is aligned first; in the UNALIGNED variant the
// n*8 bits are read densely.
func readOctetAligned(r *Reader, enc Encoding, n int) ([]byte, error) {
	if enc == Aligned {
		r.AlignToByte()
		return r.ReadOctets(n)
	}

	return r.ReadBitString(n * 8)
}
