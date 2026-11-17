// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"
	"fmt"
	"testing"
)

func TestForwardCodesRoundTrip(t *testing.T) {
	seen := map[string]bool{}

	for _, c := range forwardCodes {
		if seen[c.code] {
			t.Errorf("code %q is listed twice", c.code)
		}

		seen[c.code] = true

		if got := ForwardCodeFor(fmt.Errorf("apply: %w", c.err)); got != c.code {
			t.Errorf("ForwardCodeFor(%v) = %q, want %q", c.err, got, c.code)
		}

		if got := sentinelForForwardCode(c.code); !errors.Is(got, c.err) {
			t.Errorf("sentinelForForwardCode(%q) = %v, want %v", c.code, got, c.err)
		}
	}

	for _, err := range []error{
		ErrStateUsed, ErrLastPasswordAdmin, ErrNoPassword, ErrUserLimit, ErrOIDCDisabled, ErrOIDCChanged,
		ErrNotLinked, ErrNoUser, ErrAmbiguousEmail, ErrNoRole, ErrLinkedElsewhere,
	} {
		if ForwardCodeFor(err) == "" {
			t.Errorf("%v has no forward code, so a follower would see an internal error", err)
		}
	}
}
