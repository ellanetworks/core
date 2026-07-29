// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "testing"

// TestLabelledNameRejectsEmpty pins the minimum both users of this encoding
// declare: a type-4 IE of at least three octets, so at least one octet of content
// (TS 24.501 §9.11.2.1B for the DNN, TS 24.008 §10.5.6.1 for the APN). An empty
// value decoding to "" made an empty DNN IE indistinguishable from an absent one,
// which loses the fallback to the subscribed data network.
func TestLabelledNameRejectsEmpty(t *testing.T) {
	if name, err := ParseLabelledName(nil); err == nil {
		t.Errorf("ParseLabelledName(nil) = %q, want an error", name)
	}

	if name, err := ParseLabelledName([]byte{}); err == nil {
		t.Errorf("ParseLabelledName([]) = %q, want an error", name)
	}

	if b, err := AppendLabelledName(nil, ""); err == nil {
		t.Errorf("AppendLabelledName(\"\") = % x, want an error", b)
	}
}

// TestLabelledNameRejectsUnrepresentableLabels pins the values the dotted form
// cannot carry one-to-one: an empty label, and a label holding the separator.
// Both decoded to a name that re-encoded to different octets.
func TestLabelledNameRejectsUnrepresentableLabels(t *testing.T) {
	for _, wire := range [][]byte{
		{0x00},                       // one empty label
		{0x03, 'a', 'b', 'c', 0x00},  // a trailing empty label
		{0x03, '.', '0', '0'},        // a label carrying the separator
		{0x01, 'a', 0x00, 0x01, 'b'}, // an empty label in the middle
	} {
		if name, err := ParseLabelledName(wire); err == nil {
			t.Errorf("ParseLabelledName(% x) = %q, want an error", wire, name)
		}
	}

	for _, name := range []string{".", "..", ".abc", "abc.", "a..b"} {
		if b, err := AppendLabelledName(nil, name); err == nil {
			t.Errorf("AppendLabelledName(%q) = % x, want an error", name, b)
		}
	}
}
