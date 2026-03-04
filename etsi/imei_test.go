package etsi_test

import (
	"testing"

	"github.com/ellanetworks/core/etsi"
)

func TestIMEIFromPEI(t *testing.T) {
	tests := []struct {
		name     string
		pei      string
		expected string
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
			got, err := etsi.IMEIFromPEI(tc.pei)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}

			// Verify the result passes Luhn validation for non-empty results.
			if got != "" && !luhnValid(got) {
				t.Fatalf("result %q does not pass Luhn validation", got)
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
