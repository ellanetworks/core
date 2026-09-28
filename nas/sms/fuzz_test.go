// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"bytes"
	"slices"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func roundTrip[T interface{ MarshalBinary() ([]byte, error) }](t *testing.T, parse func([]byte) (T, error), b []byte) {
	t.Helper()

	buf := slices.Clone(b)

	msg, err := parse(buf)
	if err != nil && !nas.SoftOnly(err) {
		return
	}

	raw, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("decoded % x but would not encode: %v", b, err)
	}

	for i := range buf {
		buf[i] = 0xFF
	}

	if after, err := msg.MarshalBinary(); err != nil || !bytes.Equal(after, raw) {
		t.Fatalf("the parsed value aliases its input: % x, then % x", raw, after)
	}

	again, err := parse(raw)
	if err != nil && !nas.SoftOnly(err) {
		t.Fatalf("own encoding % x did not decode: %v (from % x)", raw, err, b)
	}

	stable, err := again.MarshalBinary()
	if err != nil {
		t.Fatalf("re-encode failed: %v", err)
	}

	if !bytes.Equal(stable, raw) {
		t.Fatalf("encoding is not idempotent\n first % x\nsecond % x", raw, stable)
	}
}

func FuzzParseCP(f *testing.F) {
	for _, s := range []string{"09010e000100079144775810065002aabb", "8904", "e9106f", "0901"} {
		f.Add(mustHex(s))
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		roundTrip(t, ParseCP, b)
	})
}

func FuzzParseRP(f *testing.F) {
	for _, s := range []string{"000100079144775810065002aabb", "01420791447758100650000104", "034241020000", "0409021603410101", "0607"} {
		f.Add(mustHex(s))
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		for _, dir := range []nas.Direction{nas.DirectionUplink, nas.DirectionDownlink} {
			parse := func(b []byte) (RPMessage, error) { return ParseRP(b, dir) }

			m, err := parse(b)
			if (m != nil) != (err == nil || nas.SoftOnly(err)) {
				t.Fatalf("ParseRP(% x, %s) = %v, %v: a message must come back exactly when the error is soft", b, dir, m, err)
			}

			roundTrip(t, parse, b)
		}
	})
}
