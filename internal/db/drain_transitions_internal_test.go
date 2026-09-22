// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"testing"
)

func TestDrainTransitionAllowed(t *testing.T) {
	for _, tc := range []struct {
		current string
		target  string
		want    bool
	}{
		{"", DrainStateDraining, true},
		{DrainStateActive, DrainStateDraining, true},
		{DrainStateDraining, DrainStateDraining, false},
		{DrainStateDrained, DrainStateDraining, false},

		{DrainStateDraining, DrainStateDrained, true},
		{"", DrainStateDrained, false},
		{DrainStateActive, DrainStateDrained, false},
		{DrainStateDrained, DrainStateDrained, false},

		{DrainStateDraining, DrainStateActive, true},
		{DrainStateDrained, DrainStateActive, true},
		{"", DrainStateActive, false},
		{DrainStateActive, DrainStateActive, false},
	} {
		t.Run(tc.current+"_to_"+tc.target, func(t *testing.T) {
			if got := drainTransitionAllowed(tc.current, tc.target); got != tc.want {
				t.Fatalf("drainTransitionAllowed(%q, %q) = %v, want %v", tc.current, tc.target, got, tc.want)
			}
		})
	}
}
