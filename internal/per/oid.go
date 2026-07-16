package per

// oidContentBytes produces the BER content octets of an object identifier value
// (Rec. ITU-T X.690 §8.19.4): the first octet is 40*arc0 + arc1 (for arc0 in
// 0..2), and each subsequent arc is base-128 with the MSB continuation bit set
// on all but the last octet of the arc.
func oidContentBytes(arcs []uint64) []byte {
	if len(arcs) < 2 {
		return nil
	}

	var out []byte

	first := arcs[0]*40 + arcs[1]

	out = appendBase128(out, first)
	for _, arc := range arcs[2:] {
		out = appendBase128(out, arc)
	}

	return out
}

// appendBase128 appends v in base-128 with the MSB continuation bit set on all
// but the last octet of the value.
func appendBase128(out []byte, v uint64) []byte {
	if v < 0x80 {
		return append(out, byte(v))
	}

	var tmp [10]byte

	n := 0
	tmp[n] = byte(v & 0x7F)
	v >>= 7

	n++
	for v > 0 {
		tmp[n] = byte(v&0x7F) | 0x80
		v >>= 7
		n++
	}
	// reverse into out
	for i := n - 1; i >= 0; i-- {
		out = append(out, tmp[i])
	}

	return out
}

// oidParseContent decodes BER content octets back into arcs.
func oidParseContent(p []byte) ([]uint64, error) {
	if len(p) == 0 {
		return nil, ErrTruncated
	}

	first := uint64(p[0])

	var arc0, arc1 uint64

	switch {
	case first < 40:
		arc0, arc1 = 0, first
	case first < 80:
		arc0, arc1 = 1, first-40
	default:
		arc0, arc1 = 2, first-80
	}

	arcs := []uint64{arc0, arc1}

	i := 1
	for i < len(p) {
		var v uint64

		for {
			if i >= len(p) {
				return nil, ErrTruncated
			}

			b := p[i]
			i++

			v = v<<7 | uint64(b&0x7F)
			if b&0x80 == 0 {
				break
			}
		}

		arcs = append(arcs, v)
	}

	return arcs, nil
}

// EncodeOID encodes an OBJECT IDENTIFIER per §24: BER content octets preceded
// by an unconstrained length determinant (semi-constrained whole number, lb=0).
func EncodeOID(w *Writer, enc Encoding, arcs []uint64) error {
	content := oidContentBytes(arcs)
	off := 0

	return EncodeLength(w, enc, 0, 0, false, int64(len(content)), func(count int64) error {
		end := off + int(count)
		writeOctetAligned(w, enc, content[off:end])
		off = end

		return nil
	})
}

// DecodeOID decodes an OBJECT IDENTIFIER per §24.
func DecodeOID(r *Reader, enc Encoding) ([]uint64, error) {
	var buf []byte

	err := DecodeLength(r, enc, 0, 0, false, func(count int64) error {
		p, err := readOctetAligned(r, enc, int(count))
		if err != nil {
			return err
		}

		buf = append(buf, p...)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return oidParseContent(buf)
}
