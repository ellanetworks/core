package per

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestStringRoundtrip(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"hello",
		"The quick brown fox",
		strings.Repeat("A", 200),
	}
	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, s := range cases {
			w := NewWriter()
			if err := EncodeString(w, enc, []byte(s)); err != nil {
				t.Errorf("enc=%v s=%q: %v", enc, s, err)
				continue
			}

			w.AlignToByte()
			r := NewReader(w.Bytes())

			got, err := DecodeString(r, enc)
			if err != nil {
				t.Errorf("enc=%v s=%q decode: %v", enc, s, err)
				continue
			}

			if string(got) != s {
				t.Errorf("enc=%v s=%q: got %q", enc, s, got)
			}
		}
	}
}

func TestKnownMultiplierStringRoundtrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ct charStringType
		s  string
	}{
		{CharNumericString, "123 456"},
		{CharPrintableString, "Hello, World!"},
		{CharVisibleString, "Hello123"},
		{CharIA5String, "Hello123!@#"},
	}
	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, c := range cases {
			w := NewWriter()
			if err := EncodeKnownMultiplierString(w, enc, c.ct, 0, 0, false, false, false, c.s); err != nil {
				t.Errorf("enc=%v ct=%v: %v", enc, c.ct, err)
				continue
			}

			w.AlignToByte()
			w.AlignToByte()
			r := NewReader(w.Bytes())

			got, err := DecodeKnownMultiplierString(r, enc, c.ct, 0, 0, false, false, false)
			if err != nil {
				t.Errorf("enc=%v ct=%v decode: %v", enc, c.ct, err)
				continue
			}

			if got != c.s {
				t.Errorf("enc=%v ct=%v: got %q want %q", enc, c.ct, got, c.s)
			}
		}
	}
}

func TestNumericStringCompaction(t *testing.T) {
	t.Parallel()
	// NumericString has 12 chars → 4 bits aligned, 4 bits unaligned.
	// "123" → values 1,2,3 (indices in " 0123456789" → 1=1,2=2,3=3)
	// Actually: '1'=code 49, mapped to index 2 in " 0123456789"
	// Wait: " 0123456789" → ' '=0, '0'=1, '1'=2, '2'=3, '3'=4
	for _, enc := range []Encoding{Aligned, Unaligned} {
		s := "123"

		w := NewWriter()
		if err := EncodeKnownMultiplierString(w, enc, CharNumericString, 0, 0, false, false, false, s); err != nil {
			t.Fatal(err)
		}

		w.AlignToByte()
		w.AlignToByte()
		r := NewReader(w.Bytes())

		got, err := DecodeKnownMultiplierString(r, enc, CharNumericString, 0, 0, false, false, false)
		if err != nil {
			t.Fatal(err)
		}

		if got != s {
			t.Errorf("enc=%v: got %q want %q", enc, got, s)
		}
	}
}

func TestKMStringFixedLength(t *testing.T) {
	t.Parallel()
	// VisibleString (SIZE (5)) = "Hello" → fixed length, no length determinant.
	for _, enc := range []Encoding{Aligned, Unaligned} {
		s := "Hello"

		w := NewWriter()
		if err := EncodeKnownMultiplierString(w, enc, CharVisibleString, 5, 5, true, true, false, s); err != nil {
			t.Fatal(err)
		}

		w.AlignToByte()
		w.AlignToByte()
		r := NewReader(w.Bytes())

		got, err := DecodeKnownMultiplierString(r, enc, CharVisibleString, 5, 5, true, true, false)
		if err != nil {
			t.Fatal(err)
		}

		if got != s {
			t.Errorf("enc=%v: got %q want %q", enc, got, s)
		}
	}
}

func TestKMStringConstrainedLength(t *testing.T) {
	t.Parallel()
	// VisibleString (SIZE (0..10)) = "Hi" → constrained length 2 (range 11 → 4 bits).
	for _, enc := range []Encoding{Aligned, Unaligned} {
		s := "Hi"

		w := NewWriter()
		if err := EncodeKnownMultiplierString(w, enc, CharVisibleString, 0, 10, true, true, false, s); err != nil {
			t.Fatal(err)
		}

		w.AlignToByte()
		r := NewReader(w.Bytes())

		got, err := DecodeKnownMultiplierString(r, enc, CharVisibleString, 0, 10, true, true, false)
		if err != nil {
			t.Fatal(err)
		}

		if got != s {
			t.Errorf("enc=%v: got %q want %q", enc, got, s)
		}
	}
}

func TestKMStringExtensible(t *testing.T) {
	t.Parallel()
	// VisibleString (SIZE (0..5), ...) = "Hello World" → out of root
	for _, enc := range []Encoding{Aligned, Unaligned} {
		s := "Hello World"

		w := NewWriter()
		if err := EncodeKnownMultiplierString(w, enc, CharVisibleString, 0, 5, true, true, true, s); err != nil {
			t.Fatal(err)
		}

		w.AlignToByte()
		r := NewReader(w.Bytes())

		got, err := DecodeKnownMultiplierString(r, enc, CharVisibleString, 0, 5, true, true, true)
		if err != nil {
			t.Fatal(err)
		}

		if got != s {
			t.Errorf("enc=%v: got %q want %q", enc, got, s)
		}
	}
}

func TestKMStringLargeRoundtrip(t *testing.T) {
	t.Parallel()
	// Large string to test fragmentation path.
	s := strings.Repeat("A", 20000)

	for _, enc := range []Encoding{Aligned, Unaligned} {
		w := NewWriter()
		if err := EncodeKnownMultiplierString(w, enc, CharVisibleString, 0, 0, false, false, false, s); err != nil {
			t.Errorf("enc=%v: %v", enc, err)
			continue
		}

		w.AlignToByte()
		r := NewReader(w.Bytes())

		got, err := DecodeKnownMultiplierString(r, enc, CharVisibleString, 0, 0, false, false, false)
		if err != nil {
			t.Errorf("enc=%v decode: %v", enc, err)
			continue
		}

		if got != s {
			t.Errorf("enc=%v: length mismatch got %d want %d", enc, len(got), len(s))
		}
	}
}

func TestGeneralizedTimeRoundtrip(t *testing.T) {
	t.Parallel()

	cases := []time.Time{
		time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC),
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, tc := range cases {
			w := NewWriter()
			if err := EncodeGeneralizedTime(w, enc, tc); err != nil {
				t.Errorf("enc=%v: %v", enc, err)
				continue
			}

			w.AlignToByte()
			r := NewReader(w.Bytes())

			got, err := DecodeGeneralizedTime(r, enc)
			if err != nil {
				t.Errorf("enc=%v decode: %v", enc, err)
				continue
			}

			if !got.Equal(tc) {
				t.Errorf("enc=%v: got %v want %v", enc, got, tc)
			}
		}
	}
}

func TestUTCTimeRoundtrip(t *testing.T) {
	t.Parallel()

	cases := []time.Time{
		time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC),
	}
	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, tc := range cases {
			w := NewWriter()
			if err := EncodeUTCTime(w, enc, tc); err != nil {
				t.Errorf("enc=%v: %v", enc, err)
				continue
			}

			w.AlignToByte()
			r := NewReader(w.Bytes())

			got, err := DecodeUTCTime(r, enc)
			if err != nil {
				t.Errorf("enc=%v decode: %v", enc, err)
				continue
			}

			if !got.Equal(tc) {
				t.Errorf("enc=%v: got %v want %v", enc, got, tc)
			}
		}
	}
}

func TestBitsPerChar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		t         charStringType
		aligned   int
		unaligned int
	}{
		{CharNumericString, 4, 4},
		{CharPrintableString, 8, 7},
		{CharVisibleString, 8, 7},
		{CharIA5String, 8, 7},
	}
	for _, c := range cases {
		if got := bitsPerChar(c.t, Aligned); got != c.aligned {
			t.Errorf("bitsPerChar(%v, Aligned) = %d, want %d", c.t, got, c.aligned)
		}

		if got := bitsPerChar(c.t, Unaligned); got != c.unaligned {
			t.Errorf("bitsPerChar(%v, Unaligned) = %d, want %d", c.t, got, c.unaligned)
		}
	}
}

func TestREALNaN(t *testing.T) {
	t.Parallel()

	content := realContentBER(math.NaN())
	if len(content) != 1 || content[0] != 0x42 {
		t.Fatalf("NaN content = %x, want [42]", content)
	}

	_, err := realParseBER(content)
	if err != nil {
		t.Fatalf("NaN parse: %v", err)
	}
}
