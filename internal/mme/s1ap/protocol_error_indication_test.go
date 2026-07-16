// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// TS 36.413 §10.2: a PDU the MME cannot decode is answered with an ERROR INDICATION
// carrying cause "transfer-syntax-error", not dropped silently.
func TestSendProtocolErrorIndication_TransferSyntax(t *testing.T) {
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
}

// TS 36.413 §10.3.4.1: an initiating message with an unknown Procedure Code is
// rejected with an ERROR INDICATION carrying cause "abstract-syntax-error-reject".
func TestRouteUnknownProcedure_SendsErrorIndication(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	// A Procedure Code no initiating-message handler serves.
	Route(m, context.Background(), mme.NewRadioForTest(cc), &s1ap.InitiatingMessage{
		ProcedureCode: s1ap.ProcedureCode(200),
	})

	if cc.count() != 1 {
		t.Fatalf("sent %d messages, want 1 (Error Indication)", cc.count())
	}

	ind := parseOutboundErrorIndication(t, cc.sent[0])

	want := s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolAbstractSyntaxErrorReject}
	if ind.Cause == nil || *ind.Cause != want {
		t.Fatalf("cause = %v, want abstract-syntax-error-reject", ind.Cause)
	}
}
