package per

import (
	"math"
	"testing"
)

func TestREALContentBER(t *testing.T) {
	t.Parallel()

	cases := []struct {
		f    float64
		want []byte
	}{
		{0, nil},
		{1, []byte{0x81, 0x00, 0x01}},   // +1.0 = 1 * 2^0
		{-1, []byte{0xC1, 0x00, 0x01}},  // -1.0
		{2, []byte{0x81, 0x01, 0x01}},   // +2.0 = 1 * 2^1
		{0.5, []byte{0x81, 0xFF, 0x01}}, // +0.5 = 1 * 2^-1
		{math.Inf(1), []byte{0x40}},
		{math.Inf(-1), []byte{0x41}},
	}
	for _, c := range cases {
		got := realContentBER(c.f)
		if len(got) != len(c.want) {
			t.Errorf("f=%v: got %x, want %x", c.f, got, c.want)
			continue
		}

		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("f=%v: got %x, want %x", c.f, got, c.want)
				break
			}
		}
	}
}

func TestREALRoundtrip(t *testing.T) {
	t.Parallel()

	values := []float64{
		0, 1, -1, 2, -2, 0.5, -0.5, 3.14, -3.14,
		1000000, 1e-10, -1e10,
		math.Inf(1), math.Inf(-1),
	}
	for _, enc := range []Encoding{Aligned, Unaligned} {
		for _, f := range values {
			w := NewWriter()
			if err := EncodeREAL(w, enc, f); err != nil {
				t.Errorf("enc=%v f=%v: %v", enc, f, err)
				continue
			}

			r := NewReader(w.Bytes())

			got, err := DecodeREAL(r, enc)
			if err != nil {
				t.Errorf("enc=%v f=%v decode: %v", enc, f, err)
				continue
			}

			if math.IsNaN(got) || math.IsNaN(f) {
				continue
			}

			if math.IsInf(f, 0) {
				if !math.IsInf(got, 0) || math.Signbit(got) != math.Signbit(f) {
					t.Errorf("enc=%v f=%v: got %v", enc, f, got)
				}

				continue
			}

			if got != f {
				t.Errorf("enc=%v f=%v: got %v", enc, f, got)
			}
		}
	}
}
