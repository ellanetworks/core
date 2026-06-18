// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

// Error Indication is rare in normal operation, so the example is built with the
// codec rather than captured.
func TestDecodeErrorIndication(t *testing.T) {
	mme := s1ap.MMEUES1APID(7)
	enb := s1ap.ENBUES1APID(807)
	ind := &s1ap.ErrorIndication{
		MMEUES1APID: &mme,
		ENBUES1APID: &enb,
		Cause:       &s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: 0},
	}

	raw, err := ind.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(raw)
	if msg.Value.Error != "" {
		t.Fatalf("decode error: %s", msg.Value.Error)
	}

	if msg.ProcedureCode.Label != "ErrorIndication" {
		t.Fatalf("proc = %q", msg.ProcedureCode.Label)
	}

	if mustIE(t, msg, idMMEUES1APID).Value != uint32(7) || mustIE(t, msg, idENBUES1APID).Value != uint32(807) {
		t.Fatal("UE id mismatch")
	}

	c := mustIE(t, msg, idCause).Value.(Cause)
	if c.Group.Label != "protocol" || c.Value.Label != "transfer-syntax-error" {
		t.Fatalf("cause = %+v", c)
	}
}
