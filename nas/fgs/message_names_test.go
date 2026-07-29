// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"strings"
	"testing"
)

// TestMessageTypeNames checks every 5GMM and 5GSM message type names itself, and
// that an unassigned one says so rather than passing for a message.
func TestMessageTypeNames(t *testing.T) {
	if got := MsgRegistrationRequest.String(); got != "REGISTRATION REQUEST" {
		t.Errorf("MsgRegistrationRequest = %q", got)
	}

	if got := MsgGMMStatus.String(); got != "5GMM STATUS" {
		t.Errorf("MsgGMMStatus = %q", got)
	}

	if got := MsgPDUSessionEstablishmentAccept.String(); got != "PDU SESSION ESTABLISHMENT ACCEPT" {
		t.Errorf("MsgPDUSessionEstablishmentAccept = %q", got)
	}

	if got := MessageType(0x00).String(); !strings.Contains(got, "unknown") {
		t.Errorf("unassigned 5GMM type = %q, want an unknown-type report", got)
	}

	if got := GSMMessageType(0x00).String(); !strings.Contains(got, "unknown") {
		t.Errorf("unassigned 5GSM type = %q, want an unknown-type report", got)
	}
}
