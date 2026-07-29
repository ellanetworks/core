// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"
	"fmt"
	"testing"
)

func TestSoftOnly(t *testing.T) {
	soft := &IEError{IEI: 0x2E, Err: ErrTruncated}
	hard := fmt.Errorf("nas: mandatory element: %w", ErrTruncated)

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, true},
		{"one soft", soft, true},
		{"joined soft", errors.Join(soft, &IEError{IEI: 0x50, Err: ErrOverflow}), true},
		{"wrapped soft", fmt.Errorf("decoding: %w", soft), true},
		{"hard", hard, false},
		{"soft joined with hard", errors.Join(soft, hard), false},
		{"framing", &Error{Op: "LV", Err: ErrTruncated}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SoftOnly(tc.err); got != tc.want {
				t.Fatalf("SoftOnly(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestIEErrorUnwraps confirms a caller can recover both the sentinel and the
// element that failed.
func TestIEErrorUnwraps(t *testing.T) {
	err := errors.Join(
		&IEError{IEI: 0x2E, Format: IETLV, Raw: []byte{0xff}, Err: ErrTruncated},
		&IEError{IEI: 0x50, Format: IETLV, Err: ErrOverflow},
	)

	if !errors.Is(err, ErrTruncated) || !errors.Is(err, ErrOverflow) {
		t.Fatal("joined IE errors must unwrap to their causes")
	}

	var ie *IEError
	if !errors.As(err, &ie) || ie.IEI != 0x2E {
		t.Fatalf("errors.As did not yield the first IEError: %+v", ie)
	}
}
