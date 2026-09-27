// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/lmf"
	"github.com/ellanetworks/core/internal/lmf/lpp"
)

func TestWriteLocationError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"UE not found", lmf.ErrNotFound, http.StatusNotFound, "UE not found"},
		{"no cell coordinate", lmf.ErrNoLocationEstimate, http.StatusNotFound, "no coordinate for serving cell"},
		{"UE provided no estimate", fmt.Errorf("LPP session failed: %w", lpp.ErrUENoLocationEstimate), http.StatusNotFound, "UE provided no location estimate"},
		{"timeout", fmt.Errorf("AGNSS positioning timed out: %w", context.DeadlineExceeded), http.StatusGatewayTimeout, "timed out"},
		{"other", errors.New("boom"), http.StatusInternalServerError, "Failed to determine location"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeLocationError(context.Background(), rec, tc.err)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}

			if !strings.Contains(rec.Body.String(), tc.message) {
				t.Errorf("body = %s, want it to contain %q", rec.Body.String(), tc.message)
			}
		})
	}
}
