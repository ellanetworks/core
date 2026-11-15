// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ap

import "testing"

func TestMessageEqualComparesPreservedState(t *testing.T) {
	base := func() Message[uint16] {
		var m Message[uint16]

		m.Append(Field[uint16]{ID: 9, Criticality: CriticalityIgnore, Value: []byte{1}})
		m.exempt = []uint16{3}
		m.absent = []absentIE[uint16]{{id: 4, encoded: "\x01"}}

		return m
	}

	if a, b := base(), base(); !a.Equal(b) {
		t.Fatal("identical messages differ")
	}

	a := base()
	a.diagnostics = Diagnostics[uint16]{RootNotComprehended: true}

	if !a.Equal(base()) {
		t.Fatal("diagnostics are derived from the preserved state and must not affect equality")
	}

	for name, mutate := range map[string]func(*Message[uint16]){
		"preserved IE": func(m *Message[uint16]) { m.Append(Field[uint16]{ID: 10, Value: []byte{2}}) },
		"exempt":       func(m *Message[uint16]) { m.exempt = nil },
		"absent":       func(m *Message[uint16]) { m.absent[0].encoded = "\x02" },
	} {
		m := base()
		mutate(&m)

		if m.Equal(base()) {
			t.Errorf("%s: messages compare equal", name)
		}
	}
}
