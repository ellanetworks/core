// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

// writeOctetAligned writes p as an octet-aligned bit-field (§11.1.4): padded to
// an octet boundary in the ALIGNED variant, packed densely in the UNALIGNED one.
func writeOctetAligned(w *Writer, enc Encoding, p []byte) {
	if enc == Aligned {
		w.AlignToByte()
		_ = w.WriteOctets(p)

		return
	}

	w.WriteBitString(p, len(p)*8)
}

// readOctetAligned reads n octets as an octet-aligned bit-field (§11.1.4).
func readOctetAligned(r *Reader, enc Encoding, n int) ([]byte, error) {
	if enc == Aligned {
		r.AlignToByte()
		return r.ReadOctets(n)
	}

	return r.ReadBitString(n * 8)
}
