// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

type failingRegistrar struct{}

func (failingRegistrar) Register(context.Context, string) (int64, error) {
	return 0, errors.New("no leader")
}

func (failingRegistrar) Confirmed(context.Context, string, int64) bool { return false }

func (failingRegistrar) Purge(string) {}

func (failingRegistrar) Reconcile(context.Context, string, func() int64, func(context.Context)) {
}

func TestActivateDefaultBearerRejectsWhenRegistrationFails(t *testing.T) {
	m := newTestMME(t)
	m.Registrations = failingRegistrar{}
	ue, cc := securedUE(t, m)

	activateDefaultBearer(context.Background(), m, ue, ue.Conn())

	if len(cc.sent) != 2 {
		t.Fatalf("expected Attach Reject + UE Context Release Command, got %d", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeProtectedDownlink(t, ue, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != eps.EMMCauseNetworkFailure {
		t.Fatalf("Attach Reject cause = %d, want %d (network failure, TS 29.272 Annex A)", rej.Cause, eps.EMMCauseNetworkFailure)
	}
}

func TestTrackingAreaUpdateAcceptedWhenRegistrationFails(t *testing.T) {
	m := newTestMME(t)
	m.Registrations = failingRegistrar{}
	ue, cc := securedUE(t, m)

	HandleNAS(context.Background(), m, ue.Conn(), trackingAreaUpdateNAS(t, ue, nil))

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	accept := decodeProtectedDownlink(t, ue, cc.sent[0])

	if mt, err := eps.PeekMessageType(accept); err != nil || mt != eps.MsgTrackingAreaUpdateAccept {
		t.Fatalf("downlink message = %#x (err %v), want TAU Accept", mt, err)
	}
}
