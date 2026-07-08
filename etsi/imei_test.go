// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package etsi_test

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
)

func TestNewIMEIFromPEI(t *testing.T) {
	tests := []struct {
		name     string
		pei      string
		expected string // normalized 15-digit IMEI
		wantErr  bool
	}{
		{
			name:     "empty",
			pei:      "",
			expected: "",
		},
		{
			name:     "imei prefix 15 digits",
			pei:      "imei-356332280764231",
			expected: "356332280764231",
		},
		{
			name:     "imeisv prefix 16 digits",
			pei:      "imeisv-3563322807642310",
			expected: "356332280764231",
		},
		{
			name:     "bare 15 digits",
			pei:      "356332280764231",
			expected: "356332280764231",
		},
		{
			name:     "bare 16 digits (imeisv without prefix)",
			pei:      "3563322807642310",
			expected: "356332280764231",
		},
		{
			name:     "known IMEI luhn check",
			pei:      "imeisv-4901234567890100",
			expected: "490123456789012",
		},
		{
			name:    "too short",
			pei:     "imei-1234",
			wantErr: true,
		},
		{
			name:    "non-digit characters",
			pei:     "imeisv-35633228076423AB",
			wantErr: true,
		},
		{
			name:    "wrong length with prefix",
			pei:     "imei-12345678901234567",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			imei, err := etsi.NewIMEIFromPEI(tc.pei)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", imei.IMEI())
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := imei.IMEI(); got != tc.expected {
				t.Fatalf("IMEI() = %q, want %q", got, tc.expected)
			}

			if imei.IsSet() != (tc.expected != "") {
				t.Fatalf("IsSet() = %v, want %v", imei.IsSet(), tc.expected != "")
			}

			// Verify the normalized IMEI passes Luhn validation.
			if got := imei.IMEI(); got != "" && !luhnValid(got) {
				t.Fatalf("IMEI() %q does not pass Luhn validation", got)
			}
		})
	}
}

// luhnValid checks that a digit string passes the Luhn algorithm.
func luhnValid(s string) bool {
	sum := 0
	double := false

	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}

		sum += d
		double = !double
	}

	return sum%10 == 0
}
