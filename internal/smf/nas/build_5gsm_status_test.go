// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"testing"

	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestBuildGSM5GSMStatus_RoundTrip(t *testing.T) {
	const (
		pduSessionID uint8 = 5
		pti          uint8 = 9
	)

	encoded, err := smfNas.BuildGSM5GSMStatus(pduSessionID, pti, nasMessage.Cause5GSMPTIMismatch)
	if err != nil {
		t.Fatalf("BuildGSM5GSMStatus failed: %v", err)
	}

	m := new(nas.Message)
	if err := m.PlainNasDecode(&encoded); err != nil {
		t.Fatalf("PlainNasDecode failed: %v", err)
	}

	if m.GsmHeader.GetMessageType() != nas.MsgTypeStatus5GSM {
		t.Fatalf("message type = %d, want %d (5GSM STATUS)", m.GsmHeader.GetMessageType(), nas.MsgTypeStatus5GSM)
	}

	if m.Status5GSM == nil {
		t.Fatal("decoded message has no Status5GSM payload")
	}

	if got := m.Status5GSM.GetPDUSessionID(); got != pduSessionID {
		t.Errorf("PDU session ID = %d, want %d", got, pduSessionID)
	}

	if got := m.Status5GSM.GetPTI(); got != pti {
		t.Errorf("PTI = %d, want %d", got, pti)
	}

	if got := m.Status5GSM.GetCauseValue(); got != nasMessage.Cause5GSMPTIMismatch {
		t.Errorf("cause = %d, want %d (#47 PTI mismatch)", got, nasMessage.Cause5GSMPTIMismatch)
	}
}
