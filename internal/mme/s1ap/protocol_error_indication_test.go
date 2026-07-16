// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// TS 36.413 §10.2 / §10.3.4.1: a PDU the MME cannot decode draws a transfer-syntax
// ERROR INDICATION, and an unknown Procedure Code an abstract-syntax-error-reject one.
func TestSendProtocolErrorIndication(t *testing.T) {
	for _, cause := range []int{
		s1ap.CauseProtocolTransferSyntaxError,
		s1ap.CauseProtocolAbstractSyntaxErrorReject,
	} {
		m := newTestMME(t)
		cc := &captureConn{}

		sendProtocolErrorIndication(m, cc, cause)

		if cc.count() != 1 {
			t.Fatalf("sent %d messages, want 1 (Error Indication)", cc.count())
		}

		ind := parseOutboundErrorIndication(t, cc.sent[0])

		want := s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: cause}
		if ind.Cause == nil || *ind.Cause != want {
			t.Fatalf("cause = %v, want protocol cause %d", ind.Cause, cause)
		}
	}
}

// TS 36.413 §10.3.4.1: an initiating message with an unknown Procedure Code is
// rejected with an ERROR INDICATION carrying cause "abstract-syntax-error-reject".
func TestUnknownProcedure_SendsErrorIndication(t *testing.T) {
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
