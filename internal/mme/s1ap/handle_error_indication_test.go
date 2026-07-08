// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

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

	handleErrorIndication(m, context.Background(), nil, initiatingValue(t, b))

	if len(cc.sent) != 1 {
		t.Fatalf("expected the referenced UE to be released, got %d S1AP messages", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

func TestErrorIndicationWithoutUEIsNoop(t *testing.T) {
	m := newTestMME(t)

	b, err := (&s1ap.ErrorIndication{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// No UE referenced: log only, no release, no panic.
	handleErrorIndication(m, context.Background(), nil, initiatingValue(t, b))
}
