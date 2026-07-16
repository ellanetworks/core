package per

import "strings"

// Known-multiplier character string types per §30.1.
type charStringType int

// Known-multiplier character string type identifiers.
const (
	// CharNumericString is the ASN.1 NumericString type.
	CharNumericString charStringType = iota
	CharPrintableString
	CharVisibleString
	CharIA5String
	CharBMPString
	CharUniversalString
)

// charSetInfo returns the minimum value, maximum value, and number of distinct
// characters in the full alphabet of the given known-multiplier type.
type charSetInfo struct {
	lb, ub uint64 // min and max code values in the alphabet
	n      int    // number of distinct characters
}

func charSet(t charStringType) charSetInfo {
	switch t {
	case CharNumericString:
		// " 0123456789" → code values 32, 48-57, but not all contiguous.
		// Range: 32-57, 12 characters.
		return charSetInfo{32, 57, 12}
	case CharPrintableString:
		// '()*+,-./0-9:=?A-Z a-z → range 32-122, but not all present.
		return charSetInfo{32, 122, 74}
	case CharVisibleString:
		// ASCII 32-126, all present.
		return charSetInfo{32, 126, 95}
	case CharIA5String:
		// ASCII 0-127, all present.
		return charSetInfo{0, 127, 128}
	case CharBMPString:
		// UCS-2: 0-65535.
		return charSetInfo{0, 65535, 65536}
	case CharUniversalString:
		// UCS-4: 0-2147483647 (2^31-1).
		return charSetInfo{0, 2147483647, 2147483648}
	default:
		return charSetInfo{0, 0, 0}
	}
}

// bitsPerChar returns the number of bits per character for the given string
// type and variant per §30.5.2.
//
//	ALIGNED: B2 = smallest power of 2 >= B, where B = ceil(log2(N))
//	UNALIGNED: B = ceil(log2(N))
func bitsPerChar(t charStringType, enc Encoding) int {
	info := charSet(t)
	if info.n <= 1 {
		return 0
	}

	b := bitsNeeded(int64(info.n))

	if enc == Aligned {
		// Round up to next power of 2.
		b2 := 1
		for b2 < b {
			b2 <<= 1
		}

		return b2
	}

	return b
}

// needsCompaction reports whether the alphabet's code values fit in b bits
// without remapping (§30.5.4a). If not, characters are remapped to 0..N-1.
func needsCompaction(t charStringType, b int) bool {
	info := charSet(t)
	maxVal := uint64(1)<<uint(b) - 1

	return info.ub > maxVal
}

// EncodeKnownMultiplierString encodes a known-multiplier character string per
// §30.5. Characters are compacted to the minimum bits per character. Size
// constraints (lb, ub) control length determinant encoding like OCTET STRING.
func EncodeKnownMultiplierString(
	w *Writer, enc Encoding,
	t charStringType,
	lb, ub int64, hasLB, hasUB, extensible bool,
	s string,
) error {
	if extensible {
		inRoot := !hasUB || int64(len(s)) <= ub
		if hasLB {
			inRoot = inRoot && int64(len(s)) >= lb
		}

		w.WriteBit(!inRoot)

		if !inRoot {
			return encodeKMStringUnconstrained(w, enc, t, s)
		}
	}

	b := bitsPerChar(t, enc)

	// Fixed length (ub == lb, ub < 64K): no length determinant (§30.5.6).
	if hasUB && hasLB && ub == lb && ub < sixtyFourK {
		encodeChars(w, enc, t, s, b, ub*int64(b) > 16)
		return nil
	}

	// Variable length with size constraint (§30.5.7).
	return encodeKMStringLen(w, enc, t, lb, ub, hasUB, s, b)
}

// encodeKMStringUnconstrained encodes with no effective size constraint and
// a semi-constrained length (§30.4 out-of-root case).
func encodeKMStringUnconstrained(w *Writer, enc Encoding, t charStringType, s string) error {
	b := bitsPerChar(t, enc)
	return encodeKMStringLen(w, enc, t, 0, 0, false, s, b)
}

// encodeKMStringLen encodes the length determinant + compacted characters.
func encodeKMStringLen(
	w *Writer, enc Encoding,
	t charStringType,
	lb, ub int64, hasUB bool,
	s string, b int,
) error {
	n := int64(len(s))
	compacted := compactChars(t, s, b)
	// §30.5.7: octet-aligned if aub*b >= 16, else not.
	alignThreshold := hasUB && ub*int64(b) >= 16

	return EncodeLength(w, enc, lb, ub, hasUB, n, func(count int64) error {
		off := int64(0)
		end := off + count

		if alignThreshold && enc == Aligned {
			w.AlignToByte()
		}

		for i := off; i < end; i++ {
			w.WriteBits(uint64(compacted[i]), b)
		}

		return nil
	})
}

// encodeChars writes characters directly without a length determinant (fixed
// length case, §30.5.6).
func encodeChars(w *Writer, enc Encoding, t charStringType, s string, b int, octetAlign bool) {
	if octetAlign && enc == Aligned {
		w.AlignToByte()
	}

	compacted := compactChars(t, s, b)
	for _, v := range compacted {
		w.WriteBits(uint64(v), b)
	}
}

// compactChars converts each character of s into its compacted numeric value
// per §30.5.4. If the alphabet fits in b bits without remapping, the original
// code value is used; otherwise characters are mapped to 0..N-1 in canonical
// order.
func compactChars(t charStringType, s string, b int) []uint32 {
	if !needsCompaction(t, b) {
		// Use original code values.
		out := make([]uint32, 0, len(s))
		for _, r := range s {
			out = append(out, uint32(r))
		}

		return out
	}
	// Build a remapping table for the full alphabet.
	table := buildCharTable(t)

	out := make([]uint32, 0, len(s))
	for _, r := range s {
		out = append(out, table[r])
	}

	return out
}

// buildCharTable builds the remapping table for a compacted alphabet per
// §30.5.4b: characters in canonical order are assigned 0, 1, 2, ...
func buildCharTable(t charStringType) map[rune]uint32 {
	chars := alphabet(t)

	table := make(map[rune]uint32, len(chars))
	for i, r := range chars {
		table[r] = uint32(i)
	}

	return table
}

// alphabet returns the characters of the full alphabet in canonical order.
func alphabet(t charStringType) []rune {
	switch t {
	case CharNumericString:
		return []rune(" 0123456789")
	case CharPrintableString:
		return []rune(" '()+,-./0123456789:=?ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	case CharVisibleString:
		var r []rune
		for c := rune(32); c <= 126; c++ {
			r = append(r, c)
		}

		return r
	case CharIA5String:
		var r []rune
		for c := rune(0); c <= 127; c++ {
			r = append(r, c)
		}

		return r
	case CharBMPString, CharUniversalString:
		// Too large for a table; we use direct code values for these.
		return nil
	}

	return nil
}

// DecodeKnownMultiplierString decodes a known-multiplier character string per
// §30.5.
func DecodeKnownMultiplierString(
	r *Reader, enc Encoding,
	t charStringType,
	lb, ub int64, hasLB, hasUB, extensible bool,
) (string, error) {
	if extensible {
		bit, err := r.ReadBit()
		if err != nil {
			return "", err
		}

		if bit {
			return decodeKMStringUnconstrained(r, enc, t)
		}
	}

	b := bitsPerChar(t, enc)

	// Fixed length (ub == lb, ub < 64K): no length determinant (§30.5.6).
	if hasUB && hasLB && ub == lb && ub < sixtyFourK {
		return decodeFixedChars(r, enc, t, int(ub), b, ub*int64(b) > 16)
	}

	// Variable length with size constraint (§30.5.7).
	return decodeKMStringLen(r, enc, t, lb, ub, hasUB, b)
}

func decodeKMStringUnconstrained(r *Reader, enc Encoding, t charStringType) (string, error) {
	b := bitsPerChar(t, enc)
	return decodeKMStringLen(r, enc, t, 0, 0, false, b)
}

func decodeKMStringLen(
	r *Reader, enc Encoding,
	t charStringType,
	lb, ub int64, hasUB bool,
	b int,
) (string, error) {
	var chars []uint32

	alignThreshold := hasUB && ub*int64(b) >= 16

	err := DecodeLength(r, enc, lb, ub, hasUB, func(count int64) error {
		if alignThreshold && enc == Aligned {
			r.AlignToByte()
		}

		for range count {
			v, err := r.ReadBits(b)
			if err != nil {
				return err
			}

			chars = append(chars, uint32(v))
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return expandChars(t, chars, b), nil
}

func decodeFixedChars(r *Reader, enc Encoding, t charStringType, n int, b int, octetAlign bool) (string, error) {
	if octetAlign && enc == Aligned {
		r.AlignToByte()
	}

	chars := make([]uint32, 0, n)
	for range n {
		v, err := r.ReadBits(b)
		if err != nil {
			return "", err
		}

		chars = append(chars, uint32(v))
	}

	return expandChars(t, chars, b), nil
}

// expandChars converts compacted numeric values back to a string per §30.5.4.
func expandChars(t charStringType, chars []uint32, b int) string {
	if !needsCompaction(t, b) {
		var sb strings.Builder
		for _, v := range chars {
			sb.WriteRune(rune(v))
		}

		return sb.String()
	}
	// Reverse the remapping.
	table := buildCharTable(t)

	reverse := make(map[uint32]rune, len(table))
	for r, v := range table {
		reverse[v] = r
	}

	var sb strings.Builder
	for _, v := range chars {
		sb.WriteRune(reverse[v])
	}

	return sb.String()
}
