// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// TS 36.413 §10.2: the transfer-syntax ERROR INDICATION for an undecodable PDU is
// cause-only.
func TestSendProtocolErrorIndication(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	sendProtocolErrorIndication(m, cc, s1ap.CauseProtocolTransferSyntaxError)

	if cc.count() != 1 {
		t.Fatalf("sent %d messages, want 1 (Error Indication)", cc.count())
	}

	ind := parseOutboundErrorIndication(t, cc.sent[0])

	want := s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolTransferSyntaxError}
	if ind.Cause == nil || *ind.Cause != want {
		t.Fatalf("cause = %v, want transfer-syntax-error", ind.Cause)
	}

	if ind.CriticalityDiagnostics != nil {
		t.Errorf("transfer-syntax Error Indication carries Criticality Diagnostics: %+v", ind.CriticalityDiagnostics)
	}
}

// TS 36.413 §10.3.4.1: an unknown Procedure Code is answered per its received
// criticality — Reject and Ignore-and-Notify draw an ERROR INDICATION with
// Criticality Diagnostics; Ignore draws no reply.
func TestUnknownProcedure_SendsErrorIndication(t *testing.T) {
	tests := []struct {
		name      string
		crit      s1ap.Criticality
		wantReply bool
		wantCause int
	}{
		{"reject", s1ap.CriticalityReject, true, s1ap.CauseProtocolAbstractSyntaxErrorReject},
		{"ignore-and-notify", s1ap.CriticalityNotify, true, s1ap.CauseProtocolAbstractSyntaxErrorIgnoreAndNotify},
		{"ignore", s1ap.CriticalityIgnore, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMME(t)
			cc := &captureConn{}

			// A Procedure Code no initiating-message handler serves.
			Route(m, context.Background(), mme.NewRadioForTest(cc), &s1ap.InitiatingMessage{
				ProcedureCode: s1ap.ProcedureCode(200),
				Criticality:   tt.crit,
			})

			if !tt.wantReply {
				if cc.count() != 0 {
					t.Fatalf("criticality ignore must draw no reply, sent %d", cc.count())
				}

				return
			}

			if cc.count() != 1 {
				t.Fatalf("sent %d messages, want 1 (Error Indication)", cc.count())
			}

			ind := parseOutboundErrorIndication(t, cc.sent[0])

			wantCause := s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: tt.wantCause}
			if ind.Cause == nil || *ind.Cause != wantCause {
				t.Fatalf("cause = %v, want protocol cause %d", ind.Cause, tt.wantCause)
			}

			cd := ind.CriticalityDiagnostics
			if cd == nil || cd.ProcedureCode == nil || *cd.ProcedureCode != s1ap.ProcedureCode(200) ||
				cd.TriggeringMessage == nil || *cd.TriggeringMessage != s1ap.TriggeringInitiatingMessage ||
				cd.ProcedureCriticality == nil || *cd.ProcedureCriticality != tt.crit {
				t.Errorf("Criticality Diagnostics = %+v, want procedure 200 / initiating / criticality %d", cd, tt.crit)
			}
		})
	}
}
