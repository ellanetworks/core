// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

type orderingSessionManager struct {
	fakeSessionManager
	onRelease func()
}

func (s *orderingSessionManager) ReleaseEPSSession(ctx context.Context, ref string, cause models.SessionReleaseRequest) error {
	if s.onRelease != nil {
		s.onRelease()
	}

	return s.fakeSessionManager.ReleaseEPSSession(ctx, ref, cause)
}

func detachingUE(t *testing.T) (*mme.MME, *mme.UeContext, *captureConn, *int) {
	t.Helper()

	sessions := &orderingSessionManager{}
	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), fakeBearerStore{}, sessions)
	m.NAS = &nasHandler{m: m}

	ue, cc := securedUE(t, m)
	testPDN(ue)

	sentAtRelease := -1
	sessions.onRelease = func() { sentAtRelease = cc.count() }

	return m, ue, cc, &sentAtRelease
}

// TS 23.401 §5.3.8.2.1
func TestUEInitiatedDetachReleasesSessionsBeforeAcceptAndSignallingRelease(t *testing.T) {
	m, ue, cc, sentAtRelease := detachingUE(t)

	HandleNAS(context.Background(), m, ue.Conn(), detachRequest(t, ue, false))

	if *sentAtRelease != 0 {
		t.Fatalf("session released after %d S1AP messages, want before the Detach Accept and the UE Context Release Command", *sentAtRelease)
	}

	if cc.count() != 2 {
		t.Fatalf("sent %d S1AP messages, want Detach Accept then UE Context Release Command", cc.count())
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// TS 23.401 §5.3.8.2.1
func TestSwitchOffDetachReleasesSessionsBeforeSignallingReleaseWithoutAccept(t *testing.T) {
	m, ue, cc, sentAtRelease := detachingUE(t)

	HandleNAS(context.Background(), m, ue.Conn(), detachRequest(t, ue, true))

	if *sentAtRelease != 0 {
		t.Fatalf("session released after %d S1AP messages, want before the UE Context Release Command", *sentAtRelease)
	}

	if cc.count() != 1 {
		t.Fatalf("sent %d S1AP messages, want only the UE Context Release Command", cc.count())
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

// TS 23.401 §5.3.8.3
func TestNetworkInitiatedDetachReleasesSessionsBeforeSignallingRelease(t *testing.T) {
	m, ue, cc, sentAtRelease := detachingUE(t)

	m.DetachSubscriber(context.Background(), testSubscriber.IMSI)

	if cc.count() != 1 {
		t.Fatalf("sent %d S1AP messages, want the Detach Request", cc.count())
	}

	handleDetachAccept(context.Background(), m, ue, ue.Conn())

	if *sentAtRelease != 1 {
		t.Fatalf("session released after %d S1AP messages, want after the Detach Request and before the UE Context Release Command", *sentAtRelease)
	}

	if cc.count() != 2 {
		t.Fatalf("sent %d S1AP messages, want Detach Request then UE Context Release Command", cc.count())
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}
