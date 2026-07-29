// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// This file pins the third round-trip property: for an input whose optional
// elements are in its message's spec-table order, MarshalBinary(Parse(b)) is b.
//
// Each case assembles a message from its mandatory part followed by every
// optional element the message models, framed as the message's own table frames
// it and ordered as the 3GPP message-content table lists it. Parsing then
// re-encoding must reproduce those octets exactly, which holds only if the
// encoder emits the elements in the order 3GPP defines. The case asserts the
// parse modelled every element too, since an element that fell through to
// Unrecognized would be re-emitted at the position it arrived in and the
// comparison would pass without testing the encoder at all.

// canonicalCase is one message's spec-table order.
type canonicalCase struct {
	// name identifies the case and names the clause the order comes from.
	name string
	// bare is the message with no optional element set; its encoding is the
	// mandatory part every case starts from.
	bare Message
	// order lists the optional elements in 3GPP message-content-table order,
	// each with the framing that table gives it.
	order []canonicalIE
	// values overrides the shared value part for an IEI this message reads
	// differently from the rest (an IEI is message-scoped).
	values map[uint8][]byte
	// downlink decodes the case as a network-originated message. It exists only
	// for DETACH REQUEST, which TS 24.301 table 9.8.1 gives one message type in
	// both directions; every other message decodes the same either way. The 5GS
	// package needs no counterpart: TS 24.501 assigns each direction its own type.
	downlink bool
}

// canonicalIE is one optional element of a message-content table.
type canonicalIE struct {
	iei    uint8
	format nas.IEFormat
}

func TestCanonicalEncoding(t *testing.T) {
	for _, tc := range canonicalCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			want := assemble(t, tc)

			dir := nas.DirectionUplink
			if tc.downlink {
				dir = nas.DirectionDownlink
			}

			msg, err := ParseMessage(want, dir)
			if err != nil {
				t.Fatalf("ParseMessage: %v", err)
			}

			if left := unrecognized(t, msg); len(left) != 0 {
				t.Fatalf("%d elements were not modelled: %+v — the case needs a value the message accepts",
					len(left), left)
			}

			got, err := msg.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}

			if !bytes.Equal(got, want) {
				t.Errorf("re-encode is not the canonical input\n got % x\nwant % x\n%s", got, want, ieiDiff(got, want))
			}
		})
	}
}

// assemble builds the canonical encoding: the mandatory part, then every
// optional element in spec-table order.
func assemble(t *testing.T, tc canonicalCase) []byte {
	t.Helper()

	out, err := tc.bare.MarshalBinary()
	if err != nil {
		t.Fatalf("encode the mandatory part: %v", err)
	}

	for _, ie := range tc.order {
		value, ok := tc.values[ie.iei]
		if !ok {
			value, ok = canonicalValues[ie.iei]
		}

		if !ok {
			t.Fatalf("no value for IEI %#02x — add one to canonicalValues", ie.iei)
		}

		out = appendIE(t, out, ie, value)
	}

	return out
}

// appendIE frames one optional element the way its table entry says, or as a
// type-1 element when the IEI carries its value in the low nibble.
func appendIE(t *testing.T, b []byte, ie canonicalIE, value []byte) []byte {
	t.Helper()

	iei := ie.iei

	if iei >= 0x80 {
		return append(b, iei|value[0]&0x0F)
	}

	switch ie.format {
	case nas.IETV3:
		return append(append(b, iei), value...)
	case nas.IETLV:
		return append(append(b, iei, uint8(len(value))), value...)
	case nas.IETLVE:
		return append(append(b, iei, uint8(len(value)>>8), uint8(len(value))), value...)
	default:
		t.Fatalf("IEI %#02x has no framing", iei)

		return nil
	}
}

// unrecognized reads a message's preserved elements.
func unrecognized(t *testing.T, msg Message) []nas.RawIE {
	t.Helper()

	field := reflect.ValueOf(msg).Elem().FieldByName("Unrecognized")
	if !field.IsValid() {
		t.Fatalf("%T has no Unrecognized field", msg)
	}

	left, ok := field.Interface().([]nas.RawIE)
	if !ok {
		t.Fatalf("%T.Unrecognized is %s", msg, field.Type())
	}

	return left
}

// ieiDiff reports the first position at which two encodings differ, to name the
// element that moved rather than leave two hex dumps to compare by eye.
func ieiDiff(got, want []byte) string {
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			return "first difference at octet " + itoa(i)
		}
	}

	return "one encoding is a prefix of the other"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var d []byte

	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}

	return string(d)
}
