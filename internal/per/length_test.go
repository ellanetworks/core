package per

import (
	"bytes"
	"testing"
)

func TestEncodeLengthFormA(t *testing.T) {
	t.Parallel()
	// §11.9.3.6 example: n=4 -> 0 0000100 = 0x04
	w := NewWriter()

	var out []byte

	err := EncodeLength(w, Aligned, 0, 0, false, 4, func(count int64) error {
		out = append(out, make([]byte, count)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x04}
	if !bytes.Equal(got, want) {
		t.Fatalf("length got %x, want %x", got, want)
	}

	if len(out) != 4 {
		t.Fatalf("emit wrote %d, want 4", len(out))
	}
}

func TestEncodeLengthFormB(t *testing.T) {
	t.Parallel()
	// §11.9.3.7 example: n=130 -> 10 000000 10000010 = 0x80 0x82
	w := NewWriter()

	err := EncodeLength(w, Aligned, 0, 0, false, 130, func(_ int64) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got := w.Bytes()

	want := []byte{0x80, 0x82}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestDecodeLengthFormAB(t *testing.T) {
	t.Parallel()
	// form a: n=4
	r := NewReader([]byte{0x04})

	var count int64

	if err := DecodeLength(r, Aligned, 0, 0, false, func(c int64) error {
		count = c
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if count != 4 {
		t.Fatalf("form a: count=%d, want 4", count)
	}

	// form b: n=130, followed by 130 bytes of content
	content := make([]byte, 130)
	for i := range content {
		content[i] = byte(i)
	}

	r = NewReader(append([]byte{0x80, 0x82}, content...))
	count = 0

	var consumed []byte

	if err := DecodeLength(r, Aligned, 0, 0, false, func(c int64) error {
		count = c

		p, err := r.ReadOctets(int(c))
		if err != nil {
			return err
		}

		consumed = append(consumed, p...)

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if count != 130 {
		t.Fatalf("form b: count=%d, want 130", count)
	}

	if !bytes.Equal(consumed, content) {
		t.Fatal("consumed content mismatch")
	}
}

func TestEncodeLengthConstrained(t *testing.T) {
	t.Parallel()
	// constrained length ub=10 lb=0 -> range 11 -> 4 bits; n=5 -> 0101
	w := NewWriter()

	err := EncodeLength(w, Aligned, 0, 10, true, 5, func(_ int64) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0b0101_0000}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}
}

func TestLengthConstrainedRoundtrip(t *testing.T) {
	t.Parallel()

	for _, enc := range []Encoding{Aligned, Unaligned} {
		for i := range 11 {
			n := int64(i)
			w := NewWriter()
			emitCount := int64(0)

			err := EncodeLength(w, enc, 0, 10, true, n, func(count int64) error {
				emitCount = count
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()
			r := NewReader(w.Bytes())
			gotCount := int64(0)

			err = DecodeLength(r, enc, 0, 10, true, func(count int64) error {
				gotCount = count
				return nil
			})
			if err != nil {
				t.Errorf("enc=%v n=%d: %v", enc, n, err)
			}

			if emitCount != n || gotCount != n {
				t.Errorf("enc=%v n=%d: emit=%d decode=%d", enc, n, emitCount, gotCount)
			}
		}
	}
}

func TestLengthFragmentationRoundtrip(t *testing.T) {
	t.Parallel()
	// n = 16384 (exactly 16K) -> fragment m=1 then final 0x00
	w := NewWriter()
	total := int64(0)

	err := EncodeLength(w, Aligned, 0, 0, false, 16384, func(count int64) error {
		total += count
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// fragment header 0xC1, then final 0x00
	got := w.Bytes()

	want := []byte{0xC1, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	if total != 16384 {
		t.Fatalf("emitted %d, want 16384", total)
	}

	// n = 16385 -> fragment 16384 then remainder 1 (form a 0x01)
	w = NewWriter()
	total = 0

	err = EncodeLength(w, Aligned, 0, 0, false, 16385, func(count int64) error {
		total += count
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got = w.Bytes()

	want = []byte{0xC1, 0x01}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	if total != 16385 {
		t.Fatalf("emitted %d, want 16385", total)
	}

	// n = 65536 -> fragment m=4 (4*16384=65536) then final 0x00
	w = NewWriter()
	total = 0

	err = EncodeLength(w, Aligned, 0, 0, false, 65536, func(count int64) error {
		total += count
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got = w.Bytes()

	want = []byte{0xC4, 0x00}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}

	if total != 65536 {
		t.Fatalf("emitted %d, want 65536", total)
	}
}

func TestLengthFragmentDecodeRoundtrip(t *testing.T) {
	t.Parallel()

	sizes := []int64{16384, 16385, 65536, 65537, 40000}
	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, n := range sizes {
			w := NewWriter()
			// build content of n zero-bytes-equivalent units (octets)
			content := make([]byte, n)
			off := 0

			err := EncodeLength(w, enc, 0, 0, false, n, func(count int64) error {
				end := off + int(count)
				writeOctetAligned(w, enc, content[off:end])
				off = end

				return nil
			})
			if err != nil {
				t.Fatalf("enc=%v n=%d encode: %v", enc, n, err)
			}

			buf := w.Bytes()
			r := NewReader(buf)
			got := []byte{}

			err = DecodeLength(r, enc, 0, 0, false, func(count int64) error {
				p, err := readOctetAligned(r, enc, int(count))
				if err != nil {
					return err
				}

				got = append(got, p...)

				return nil
			})
			if err != nil {
				t.Fatalf("enc=%v n=%d decode: %v", enc, n, err)
			}

			if int64(len(got)) != n {
				t.Errorf("enc=%v n=%d: decoded %d bytes", enc, n, len(got))
			}
		}
	}
}
