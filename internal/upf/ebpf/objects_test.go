// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import "testing"

func TestParseCPUList(t *testing.T) {
	cases := []struct {
		list string
		want uint32
	}{
		{list: "0", want: 1},
		{list: "0-3", want: 4},
		{list: "0-1,4", want: 3},
		{list: "0-3,8,10-11", want: 7},
	}

	for _, tc := range cases {
		got, err := parseCPUList(tc.list)
		if err != nil {
			t.Errorf("%q: %v", tc.list, err)

			continue
		}

		if got != tc.want {
			t.Errorf("%q = %d, want %d", tc.list, got, tc.want)
		}
	}

	for _, list := range []string{"", "foo", "1-", "3-1"} {
		if _, err := parseCPUList(list); err == nil {
			t.Errorf("%q: accepted invalid list", list)
		}
	}
}
