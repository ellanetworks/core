// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Known-multiplier character string types (§30.1).
type charStringType int

const (
	CharNumericString charStringType = iota
	CharPrintableString
	CharVisibleString
	CharIA5String
	CharBMPString
	CharUniversalString
)

type charSetInfo struct {
	lb, ub uint64 // lowest and highest code value in the alphabet
	n      int    // number of distinct characters
}

func charSet(t charStringType) charSetInfo {
	switch t {
	case CharNumericString:
		// " 0123456789": non-contiguous within 32-57.
		return charSetInfo{32, 57, 11}
	case CharPrintableString:
		// " '()+,-./0-9:=?A-Za-z": non-contiguous within 32-122.
		return charSetInfo{32, 122, 74}
	case CharVisibleString:
		return charSetInfo{32, 126, 95}
	case CharIA5String:
		return charSetInfo{0, 127, 128}
	case CharBMPString:
		// UCS-2.
		return charSetInfo{0, 65535, 65536}
	case CharUniversalString:
		// UCS-4.
		return charSetInfo{0, 4294967295, 4294967296}
	default:
		return charSetInfo{0, 0, 0}
	}
}

// bitsPerChar returns the bits per character (§30.5.2): ceil(log2(N)) for
// UNALIGNED, rounded up to a power of two for ALIGNED.
func bitsPerChar(t charStringType, enc Encoding) int {
	info := charSet(t)
	if info.n <= 1 {
		return 0
	}

	b := bitsNeeded(int64(info.n))

	if enc == Aligned {
		b2 := 1
		for b2 < b {
			b2 <<= 1
		}

		return b2
	}

	return b
}

// needsCompaction reports whether the alphabet must be remapped to 0..N-1
// because its code values do not fit in b bits (§30.5.4a).
func needsCompaction(t charStringType, b int) bool {
	info := charSet(t)
	maxVal := uint64(1)<<uint(b) - 1

	return info.ub > maxVal
}

// EncodeKnownMultiplierString encodes a known-multiplier character string
// (§30.5), compacting characters to the minimum bits per character.
func EncodeKnownMultiplierString(
	w *Writer, enc Encoding,
	t charStringType,
	lb, ub int64, hasLB, hasUB, extensible bool,
	s string,
) error {
	if extensible {
		// The size constraint counts characters, not the UTF-8 bytes of s.
		n := int64(utf8.RuneCountInString(s))

		inRoot := !hasUB || n <= ub
		if hasLB {
			inRoot = inRoot && n >= lb
		}

		w.WriteBit(!inRoot)

		if !inRoot {
			return encodeKMStringUnconstrained(w, enc, t, s)
		}
	}

	b := bitsPerChar(t, enc)

	// §30.5.6: a fixed length needs no length determinant.
	if hasUB && hasLB && ub == lb && ub < sixtyFourK {
		return encodeChars(w, enc, t, s, b, ub*int64(b) > 16)
	}

	return encodeKMStringLen(w, enc, t, lb, ub, hasUB, s, b)
}

// encodeKMStringUnconstrained encodes the §30.4 out-of-root case: no effective
// size constraint, semi-constrained length.
func encodeKMStringUnconstrained(w *Writer, enc Encoding, t charStringType, s string) error {
	b := bitsPerChar(t, enc)
	return encodeKMStringLen(w, enc, t, 0, 0, false, s, b)
}

func encodeKMStringLen(
	w *Writer, enc Encoding,
	t charStringType,
	lb, ub int64, hasUB bool,
	s string, b int,
) error {
	compacted, err := compactChars(t, s, b)
	if err != nil {
		return err
	}
	// The length determinant counts characters, not the UTF-8 bytes of s.
	n := int64(len(compacted))
	// §30.5.7: octet-align only when ub*b ≥ 16.
	alignThreshold := hasUB && ub*int64(b) >= 16

	off := int64(0)

	return EncodeLength(w, enc, lb, ub, hasUB, n, func(count int64) error {
		end := off + count

		if alignThreshold && enc == Aligned {
			w.AlignToByte()
		}

		for i := off; i < end; i++ {
			w.WriteBits(uint64(compacted[i]), b)
		}

		off = end

		return nil
	})
}

// encodeChars writes characters with no length determinant (§30.5.6).
func encodeChars(w *Writer, enc Encoding, t charStringType, s string, b int, octetAlign bool) error {
	compacted, err := compactChars(t, s, b)
	if err != nil {
		return err
	}

	if octetAlign && enc == Aligned {
		w.AlignToByte()
	}

	for _, v := range compacted {
		w.WriteBits(uint64(v), b)
	}

	return nil
}

// compactChars converts each character of s to its compacted numeric value
// (§30.5.4): the original code value where the alphabet fits in b bits,
// otherwise its position 0..N-1 in canonical order.
func compactChars(t charStringType, s string, b int) ([]uint32, error) {
	info := charSet(t)

	if !needsCompaction(t, b) {
		out := make([]uint32, 0, len(s))

		for _, r := range s {
			if r < 0 || uint64(r) > info.ub || uint64(r) < info.lb {
				return nil, fmt.Errorf("%w: character %q outside the permitted alphabet", ErrOverflow, r)
			}

			out = append(out, uint32(r))
		}

		return out, nil
	}

	table := buildCharTable(t)

	out := make([]uint32, 0, len(s))

	for _, r := range s {
		v, ok := table[r]
		if !ok {
			return nil, fmt.Errorf("%w: character %q outside the permitted alphabet", ErrOverflow, r)
		}

		out = append(out, v)
	}

	return out, nil
}

// buildCharTable assigns 0, 1, 2, ... to the alphabet in canonical order
// (§30.5.4b).
func buildCharTable(t charStringType) map[rune]uint32 {
	chars := alphabet(t)

	table := make(map[rune]uint32, len(chars))
	for i, r := range chars {
		table[r] = uint32(i)
	}

	return table
}

// alphabet returns the full alphabet in canonical order.
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
		// Too large to tabulate; these always use direct code values.
		return nil
	}

	return nil
}

// DecodeKnownMultiplierString decodes a known-multiplier character string
// (§30.5).
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

	// §30.5.6: a fixed length needs no length determinant.
	if hasUB && hasLB && ub == lb && ub < sixtyFourK {
		return decodeFixedChars(r, enc, t, int(ub), b, ub*int64(b) > 16)
	}

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

// expandChars converts compacted numeric values back to a string (§30.5.4).
func expandChars(t charStringType, chars []uint32, b int) string {
	if !needsCompaction(t, b) {
		var sb strings.Builder
		for _, v := range chars {
			sb.WriteRune(rune(v))
		}

		return sb.String()
	}

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
