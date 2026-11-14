// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"
)

func TestSetupRequestRefIsTheGlobalRANNodeID(t *testing.T) {
	req := outcomeRequest(t, "001", "01", 0x000001)
	req.RANNodeName = nil

	if got, want := setupRequestRef(req, nil), "gNB:001-01:000102@24"; got != want {
		t.Fatalf("setupRequestRef = %q, want %q", got, want)
	}
}

func TestSetupRequestRefIsEmptyOnAParseFailure(t *testing.T) {
	if got := setupRequestRef(nil, errors.New("decode failed")); got != "" {
		t.Fatalf("setupRequestRef = %q, want empty", got)
	}
}
