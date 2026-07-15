// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import (
	"bytes"
	"testing"
)

func TestOpenTypeKnownVectors(t *testing.T) {
	cases := []struct {
		inner []byte
		want  []byte
	}{
		{[]byte{0xaa, 0xbb}, []byte{0x02, 0xaa, 0xbb}},
		{[]byte{}, []byte{0x01, 0x00}}, // X.691: minimum one octet
		{bytes.Repeat([]byte{0x7e}, 200), append([]byte{0x80, 0xc8}, bytes.Repeat([]byte{0x7e}, 200)...)},
	}
	for _, c := range cases {
		var w Writer
		if err := w.WriteOpenType(c.inner); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(w.Bytes(), c.want) {
			t.Fatalf("bytes = % x, want % x", w.Bytes(), c.want)
		}

		got, err := NewReader(w.Bytes()).ReadOpenType()
		if err != nil {
			t.Fatal(err)
		}

		want := c.inner
		if len(want) == 0 {
			want = []byte{0x00}
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("decoded % x, want % x", got, want)
		}
	}
}

func TestOpenTypeNestedRoundTrip(t *testing.T) {
	// Encode an inner value, wrap it, then unwrap and decode it back.
	var inner Writer
	if err := inner.WriteConstrainedInt(42, 0, 255); err != nil {
		t.Fatal(err)
	}

	var w Writer
	if err := w.WriteOpenType(inner.Bytes()); err != nil {
		t.Fatal(err)
	}

	raw, err := NewReader(w.Bytes()).ReadOpenType()
	if err != nil {
		t.Fatal(err)
	}

	v, err := NewReader(raw).ReadConstrainedInt(0, 255)
	if err != nil {
		t.Fatal(err)
	}

	if v != 42 {
		t.Fatalf("decoded %d, want 42", v)
	}
}
