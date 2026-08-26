// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestDecodeForwardError_PreservesCode(t *testing.T) {
	body, err := json.Marshal(ProposeForwardErrorBody{
		Message: "join token already consumed",
		Code:    ForwardCodeTokenConsumed,
	})
	if err != nil {
		t.Fatal(err)
	}

	decoded := decodeForwardError(body, http.StatusConflict)

	if got := ForwardErrorCode(decoded); got != ForwardCodeTokenConsumed {
		t.Fatalf("ForwardErrorCode = %q, want %q", got, ForwardCodeTokenConsumed)
	}

	if decoded.Error() != "join token already consumed" {
		t.Fatalf("message not preserved: %q", decoded.Error())
	}
}

func TestDecodeForwardError_OutcomeUnknownStillWins(t *testing.T) {
	body, _ := json.Marshal(ProposeForwardErrorBody{
		Message: "may have committed",
		Code:    ForwardCodeOutcomeUnknown,
	})

	if !errors.Is(decodeForwardError(body, http.StatusConflict), ErrOutcomeUnknown) {
		t.Fatal("outcome_unknown must still map to ErrOutcomeUnknown")
	}
}

func TestForwardErrorCode_UncodedIsEmpty(t *testing.T) {
	body, _ := json.Marshal(ProposeForwardErrorBody{Message: "apply failed"})

	if got := ForwardErrorCode(decodeForwardError(body, http.StatusInternalServerError)); got != "" {
		t.Fatalf("ForwardErrorCode = %q, want empty", got)
	}
}
