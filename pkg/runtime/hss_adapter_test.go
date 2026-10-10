// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/hss"
)

func TestHSSErrorMarksOnlyTransientRaftErrorsUnavailable(t *testing.T) {
	cases := map[error]bool{
		fmt.Errorf("couldn't advance the IMS sequence number: %w", db.ErrOutcomeUnknown): true,
		fmt.Errorf("cas: %w", db.ErrProposeTimeout):                                      true,
		fmt.Errorf("couldn't advance the IMS sequence number: %w", db.ErrNotFound):       false,
		errors.New("disk I/O error"):                                                     false,
	}

	for err, want := range cases {
		got := hssError(err)

		if errors.Is(got, hss.ErrUnavailable) != want || !errors.Is(got, err) {
			t.Errorf("hssError(%v) = %v, want unavailable %v and the original error kept", err, got, want)
		}
	}

	if hssError(nil) != nil {
		t.Error("hssError(nil) is not nil")
	}
}
