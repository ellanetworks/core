package per

import "testing"

// trival implements Marshaler/Unmarshaler by hand to exercise dispatch and the
// Encoding selection without depending on pergen. It encodes a single bit.
type trivial bool

func (t *trivial) MarshalPER(w *Writer, _ Encoding) error {
	w.WriteBit(bool(*t))
	return nil
}

func (t *trivial) UnmarshalPER(r *Reader, _ Encoding) error {
	b, err := r.ReadBit()
	if err != nil {
		return err
	}

	*t = trivial(b)

	return nil
}

func TestMarshalDefaultAligned(t *testing.T) {
	t.Parallel()

	v := trivial(true)

	b, err := Marshal(&v)
	if err != nil {
		t.Fatal(err)
	}
	// One bit -> single octet 0x80
	if got := b; len(got) != 1 || got[0] != 0x80 {
		t.Fatalf("got %x, want [80]", got)
	}
}

func TestUnmarshalDefaultAligned(t *testing.T) {
	t.Parallel()

	var v trivial
	if err := Unmarshal([]byte{0x80}, &v); err != nil {
		t.Fatal(err)
	}

	if !v {
		t.Fatal("decoded false, want true")
	}
}

func TestMarshalUnalignedRoundtrip(t *testing.T) {
	t.Parallel()

	v := trivial(true)

	b, err := Marshal(&v, Unaligned)
	if err != nil {
		t.Fatal(err)
	}

	var got trivial
	if err := Unmarshal(b, &got, Unaligned); err != nil {
		t.Fatal(err)
	}

	if !got {
		t.Fatal("roundtrip lost value")
	}
}

func TestEncodingString(t *testing.T) {
	t.Parallel()

	if Aligned.String() != "aligned" {
		t.Fatalf("Aligned = %q", Aligned.String())
	}

	if Unaligned.String() != "unaligned" {
		t.Fatalf("Unaligned = %q", Unaligned.String())
	}

	if Encoding(99).String() != "unknown" {
		t.Fatal("unknown encoding string")
	}
}
