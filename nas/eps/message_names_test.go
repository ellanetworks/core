// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"strings"
	"testing"
)

// TestMessageTypeNames checks the EMM and ESM message types name themselves, and
// that an unassigned one says so rather than passing for a message.
func TestMessageTypeNames(t *testing.T) {
	if got := MsgAttachRequest.String(); got != "ATTACH REQUEST" {
		t.Errorf("MsgAttachRequest = %q", got)
	}

	if got := MsgEMMStatus.String(); got != "EMM STATUS" {
		t.Errorf("MsgEMMStatus = %q", got)
	}

	if got := MsgActivateDefaultEPSBearerContextRequest.String(); got != "ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST" {
		t.Errorf("MsgActivateDefaultEPSBearerContextRequest = %q", got)
	}

	if got := MessageType(0x00).String(); !strings.Contains(got, "unknown") {
		t.Errorf("unassigned EMM type = %q, want an unknown-type report", got)
	}

	if got := ESMMessageType(0x00).String(); !strings.Contains(got, "unknown") {
		t.Errorf("unassigned ESM type = %q, want an unknown-type report", got)
	}
}
