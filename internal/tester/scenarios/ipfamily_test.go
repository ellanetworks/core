// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import "testing"

func TestParseIPFamily(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    IPFamily
		wantErr bool
	}{
		{"", IPv4Only, false},
		{"ipv4", IPv4Only, false},
		{"IPv6", IPv6Only, false},
		{" dualstack ", DualStack, false},
		{"both", DualStack, false},
		{"v6", IPv6Only, false},
		{"ip6", IPv4Only, true},
		{"dual-stack", IPv4Only, true},
		{"6", IPv4Only, true},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseIPFamily(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseIPFamily(%q) error = %v, want error %v", tc.in, err, tc.wantErr)
			}

			if !tc.wantErr && got != tc.want {
				t.Errorf("ParseIPFamily(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
