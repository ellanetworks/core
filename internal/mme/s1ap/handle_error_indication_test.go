// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func TestErrorIndicationReleasesReferencedUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	id := ue.Conn().MMEUES1APID
	cause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}

	b, err := (&s1ap.ErrorIndication{MMEUES1APID: &id, Cause: &cause}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleErrorIndication(m, context.Background(), &mme.Radio{Conn: cc}, initiatingValue(t, b))

	if len(cc.sent) != 1 {
		t.Fatalf("expected the referenced UE to be released, got %d S1AP messages", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

func TestErrorIndicationWithoutUEIsNoop(t *testing.T) {
	m := newTestMME(t)

	b, err := (&s1ap.ErrorIndication{
		Cause: s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 0}),
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	conn := &captureConn{}

	handleErrorIndication(m, context.Background(), &mme.Radio{Conn: conn}, initiatingValue(t, b))

	if got := conn.count(); got != 0 {
		t.Fatalf("an Error Indication naming no UE must release nothing, got %d S1AP messages", got)
	}
}

func TestErrorIndicationFromAnotherENBDoesNotRelease(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	id := ue.Conn().MMEUES1APID
	cause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}

	b, err := (&s1ap.ErrorIndication{MMEUES1APID: &id, Cause: &cause}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	other := &captureConn{}
	handleErrorIndication(m, context.Background(), &mme.Radio{Conn: other}, initiatingValue(t, b))

	if len(cc.sent) != 0 {
		t.Fatalf("UE released by a foreign eNB: %d S1AP messages sent", len(cc.sent))
	}
}
