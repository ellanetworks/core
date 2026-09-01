// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
)

// TS 23.401 §5.3.5
func TestReleaseUEContextAfterAnEarlierReleaseCompleted(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.TransitionTo(EMMRegistered)

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)

	if len(cc.sent) != 1 {
		t.Fatalf("first release sent %d messages, want 1 UE Context Release Command", len(cc.sent))
	}

	m.FreeUeConn(ue)

	second := &captureConn{}
	m.AttachUeConn(ue, m.NewUeConn(second, 8))

	m.ReleaseUEContext(context.Background(), ue, CauseNASNormalRelease)

	if len(second.sent) != 1 {
		t.Fatalf("second release sent %d messages, want 1 UE Context Release Command: the claim from the first release outlived the connection it claimed, so nothing is ever commanded again", len(second.sent))
	}
}

// TS 23.401 §5.3.5: the release of a superseded S1 connection buffers the downlink of
// the bearers it was carrying, so data arriving meanwhile pages the UE instead of being
// forwarded to an eNB that is releasing it.
func TestAttachUeConn_DeactivatesTheSupersededConnectionsUserPlane(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).Apn = "internet"

	fake := m.Session.(*fakeSessionManager)
	fake.deactivated = false

	second := m.NewUeConn(&captureConn{}, 9)
	m.AttachUeConn(ue, second)

	if !fake.deactivated {
		t.Error("the superseded connection's user plane was left pointing at a released eNB context")
	}
}
