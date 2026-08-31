// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// TS 36.413 §10.4
func TestHandleParseError_EmitsErrorIndication(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(cc), []byte{0xff, 0xff, 0xff})

	if cc.count() != 1 {
		t.Fatalf("expected 1 Error Indication, got %d", cc.count())
	}

	ind := parseOutboundErrorIndication(t, cc.sent[0])

	if ind.Cause == nil || ind.Cause.Group != s1ap.CauseGroupProtocol {
		t.Errorf("cause = %+v, want protocol group", ind.Cause)
	}

	cd := ind.CriticalityDiagnostics
	if cd == nil {
		t.Fatal("Error Indication carried no Criticality Diagnostics")
	}

	if cd.ProcedureCode == nil || *cd.ProcedureCode != s1ap.ProcUEContextReleaseRequest {
		t.Errorf("diagnostics procedure-code = %v, want %d", cd.ProcedureCode, s1ap.ProcUEContextReleaseRequest)
	}

	if cd.TriggeringMessage == nil || *cd.TriggeringMessage != s1ap.TriggeringInitiatingMessage {
		t.Errorf("diagnostics triggering-message = %v, want initiating", cd.TriggeringMessage)
	}
}

func TestNonAttachInitialUEMessageCreatesNoContext(t *testing.T) {
	m := newTestMME(t)

	emmStatus := []byte{0x07, 0x60, 0x00}
	for i := 0; i < 100; i++ {
		HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(nil), initiatingValue(t, initialUEMessagePDU(t, s1ap.ENBUES1APID(1000+i), emmStatus)))
	}

	if got := m.ConnCountForTest(); got != 0 {
		t.Fatalf("non-Attach Initial UE Messages left %d connections, want 0", got)
	}
}
