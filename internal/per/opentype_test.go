package per

import (
	"bytes"
	"testing"
)

func TestOpenTypeRoundtrip(t *testing.T) {
	t.Parallel()
	// Use the trivial type (1 bit: true/false) as the open-type value.
	for _, v := range []trivial{true, false} {
		for _, enc := range []Encoding{Aligned, Unaligned} {
			w := NewWriter()

			err := EncodeOpenType(w, enc, &v)
			if err != nil {
				t.Fatalf("enc=%v: %v", enc, err)
			}

			buf := w.Bytes()
			r := NewReader(buf)

			var got trivial
			if err := DecodeOpenType(r, enc, &got); err != nil {
				t.Fatalf("enc=%v decode: %v", enc, err)
			}

			if got != v {
				t.Fatalf("enc=%v: got %v, want %v", enc, got, v)
			}
		}
	}
}

func TestSkipOpenType(t *testing.T) {
	t.Parallel()

	v := trivial(true)
	w := NewWriter()
	_ = EncodeOpenType(w, Aligned, &v)
	buf := w.Bytes()

	r := NewReader(buf)
	if err := SkipOpenType(r, Aligned); err != nil {
		t.Fatal(err)
	}

	if !r.EOF() {
		t.Fatal("expected EOF after skip")
	}
}

func TestNormallySmallLengthSmall(t *testing.T) {
	t.Parallel()
	// n=3 -> bit 0 + 6-bit (3-1=2) = 0_000010
	w := NewWriter()

	var gotCount int64

	err := EncodeNormallySmallLength(w, Aligned, 3, func(count int64) error {
		gotCount = count
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0b0_000010_0}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}

	if gotCount != 3 {
		t.Fatalf("count = %d, want 3", gotCount)
	}

	r := NewReader(got)

	var decCount int64

	err = DecodeNormallySmallLength(r, Aligned, func(count int64) error {
		decCount = count
		return nil
	})
	if err != nil || decCount != 3 {
		t.Fatalf("decode: count=%d err=%v", decCount, err)
	}
}

func TestNormallySmallLengthOne(t *testing.T) {
	t.Parallel()
	// n=1 -> bit 0 + 6-bit 0 = 0_000000
	w := NewWriter()

	err := EncodeNormallySmallLength(w, Aligned, 1, func(int64) error { return nil })
	if err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0b0_000000_0}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}
}

func TestNormallySmallLengthRoundtrip(t *testing.T) {
	t.Parallel()

	for _, enc := range []Encoding{Aligned, Unaligned} {
		for n := int64(1); n <= 65; n++ {
			w := NewWriter()
			emitCount := int64(0)

			err := EncodeNormallySmallLength(w, enc, n, func(count int64) error {
				emitCount = count
				return nil
			})
			if err != nil {
				t.Fatalf("enc=%v n=%d: %v", enc, n, err)
			}

			w.AlignToByte()
			r := NewReader(w.Bytes())
			decCount := int64(0)

			err = DecodeNormallySmallLength(r, enc, func(count int64) error {
				decCount = count
				return nil
			})
			if err != nil || decCount != n || emitCount != n {
				t.Errorf("enc=%v n=%d: emit=%d dec=%d err=%v", enc, n, emitCount, decCount, err)
			}
		}
	}
}
