// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pergen

import "testing"

func TestReceiverName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"Message", "message"},
		{"Foo", "foo"},
		{"", "v"},
		{"ABC", "aBC"},
	}

	for _, c := range cases {
		if got := receiverName(c.in); got != c.want {
			t.Errorf("receiverName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStructTagGet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tag, key, want string
	}{
		{`per:"INTEGER,range:0..3" json:"version"`, "per", "INTEGER,range:0..3"},
		{`json:"version" per:"optional"`, "per", "optional"},
		{`json:"version"`, "per", ""},
		{``, "per", ""},
		{`per:""`, "per", ""},
	}

	for _, c := range cases {
		if got := structTagGet(c.tag, c.key); got != c.want {
			t.Errorf("structTagGet(%q, %q) = %q, want %q", c.tag, c.key, got, c.want)
		}
	}
}
