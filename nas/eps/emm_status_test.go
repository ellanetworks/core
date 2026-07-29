// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

func TestEMMStatusRoundTrip(t *testing.T) {
	b, err := (&EMMStatus{Cause: 111}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseEMMStatus(b)
	if err != nil {
		t.Fatal(err)
	}

	if got.Cause != 111 {
		t.Fatalf("EMMCause = %d, want 111", got.Cause)
	}
}
