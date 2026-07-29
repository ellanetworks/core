// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// encoder is satisfied by every message and information element in this package.
type encoder interface {
	MarshalBinary() ([]byte, error)
}

// roundTrip asserts the canonicalization property on any input that decodes: the
// value re-encodes, the encoding re-decodes to an equal value, and encoding that
// value again yields the same octets.
//
// It deliberately does not require MarshalBinary(Parse(b)) to equal b. An input whose
// optional elements are not in its message's spec-table order canonicalizes on
// the first encode; from there on everything must be stable, and nothing may be
// lost. reflect.DeepEqual is the right comparison because it distinguishes a nil
// slice from an empty one, which is how an absent element differs from one
// present with a zero-length value.
func roundTrip[T encoder](t *testing.T, name string, parse func([]byte) (T, error), b []byte) {
	t.Helper()

	// A soft error leaves a usable message whose failed elements are preserved,
	// so the round-trip properties still have to hold for it.
	//
	// The parse reads a copy: the aliasing check below overwrites the buffer it
	// was given, and a fuzzing engine reports the input it handed over, so
	// scribbling on that one would save a corpus entry that reproduces nothing.
	buf := slices.Clone(b)

	msg, err := parse(buf)
	if err != nil && !nas.SoftOnly(err) {
		return
	}

	// A parsed value owns its memory: overwriting the buffer it came from must
	// not change it. Anything that kept a sub-slice of the input shows up here.
	for i := range buf {
		buf[i] = 0xFF
	}

	raw, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: decoded % x but would not encode: %v", name, b, err)
	}

	// MarshalBinary does not mutate the message: the same value encodes to the
	// same octets however often it is asked.
	repeat, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: re-encoding the same value failed: %v", name, err)
	}

	if !bytes.Equal(repeat, raw) {
		t.Fatalf("%s: encoding mutated the message\n first % x\nsecond % x", name, raw, repeat)
	}

	again, err := parse(raw)
	if err != nil && !nas.SoftOnly(err) {
		t.Fatalf("%s: own encoding % x did not decode: %v (from % x)", name, raw, err, b)
	}

	stable, err := again.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: re-encode failed: %v", name, err)
	}

	if !bytes.Equal(stable, raw) {
		t.Fatalf("%s: encoding is not idempotent\n first % x\nsecond % x", name, raw, stable)
	}

	// Grouping mutates the messages, so it runs only once the octets are settled.
	groupPreserved(msg)
	groupPreserved(again)

	if !reflect.DeepEqual(msg, again) {
		t.Fatalf("%s: value changed across a round trip\n got %+v\nwant %+v\nvia % x", name, again, msg, raw)
	}
}

// groupPreserved stably sorts a message's preserved elements by the element they
// follow. Encoding puts each preserved element back after its anchor, so a
// sender that interleaved preserved and modelled elements out of the message's
// spec-table order sees them regrouped; grouping both sides the same way lets
// the comparison test what must hold — that no element is lost or altered —
// without asserting an arrival order the encoder is entitled to canonicalize.
func groupPreserved(msg any) {
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	f := v.FieldByName("Unrecognized")
	if !f.IsValid() || f.Type() != reflect.TypeOf([]nas.RawIE(nil)) {
		return
	}

	ies, _ := f.Interface().([]nas.RawIE)
	sort.SliceStable(ies, func(i, j int) bool { return ies[i].After < ies[j].After })
}
