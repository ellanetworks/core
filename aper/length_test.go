// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import (
	"bytes"
	"errors"
	"testing"
)

func TestLengthKnownVectors(t *testing.T) {
	cases := []struct {
		n    int
		want []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x80}},
		{255, []byte{0x80, 0xff}},
		{16383, []byte{0xbf, 0xff}},
	}
	for _, c := range cases {
		var w Writer
		if err := w.WriteLength(c.n); err != nil {
			t.Fatalf("n=%d: %v", c.n, err)
		}

		if !bytes.Equal(w.Bytes(), c.want) {
			t.Fatalf("n=%d: bytes = % x, want % x", c.n, w.Bytes(), c.want)
		}

		got, err := NewReader(w.Bytes()).ReadLength()
		if err != nil {
			t.Fatalf("n=%d decode: %v", c.n, err)
		}

		if got != c.n {
			t.Fatalf("n=%d: decoded %d", c.n, got)
		}
	}
}

func TestLengthFragmented(t *testing.T) {
	var w Writer
	if err := w.WriteLength(16384); !errors.Is(err, ErrFragmented) {
		t.Fatalf("expected ErrFragmented, got %v", err)
	}
}

func TestConstrainedLengthRoundTrip(t *testing.T) {
	cases := []struct{ n, lb, ub int }{
		{0, 0, 0},
		{1, 1, 1},
		{3, 1, 10},
		{200, 0, 1000},
		{5, 1, 65535},
		{12000, 0, 100000},
	}
	for _, c := range cases {
		var w Writer
		if err := w.WriteConstrainedLength(c.n, c.lb, c.ub); err != nil {
			t.Fatalf("%+v: %v", c, err)
		}

		got, err := NewReader(w.Bytes()).ReadConstrainedLength(c.lb, c.ub)
		if err != nil {
			t.Fatalf("%+v decode: %v", c, err)
		}

		if got != c.n {
			t.Fatalf("%+v: decoded %d", c, got)
		}
	}
}
