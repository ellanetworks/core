// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func inactivityRelease(ue *mme.UeContext) *s1ap.UEContextReleaseRequest {
	return &s1ap.UEContextReleaseRequest{
		MMEUES1APID: ue.Conn().MMEUES1APID,
		ENBUES1APID: 7,
		Cause:       s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUserInactivity}),
	}
}

func deliverReleaseRequest(t *testing.T, m *mme.MME, cc *captureConn, req *s1ap.UEContextReleaseRequest) {
	t.Helper()

	b, _ := req.Marshal()

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	handleUEContextReleaseRequest(m, context.Background(), mme.NewRadioForTest(cc), pdu.(*s1ap.InitiatingMessage).Value)
}

func TestUserInactivityIsDeferredWhileAnMTDeliveryIsInProgress(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	ue.SetPagedBearerForTest(5, nil)
	ue.PagingAnswered()

	deliverReleaseRequest(t, m, cc, inactivityRelease(ue))

	if len(cc.sent) != 0 {
		t.Fatalf("UE Context Release Commands sent = %d, want 0: the MME is aware of pending MT traffic (TS 23.401 5.3.5)", len(cc.sent))
	}

	ue.PagingDelivered()

	if len(cc.sent) != 1 {
		t.Fatalf("UE Context Release Commands sent = %d, want 1: the deferred S1 release never resumes once the MT delivery settles", len(cc.sent))
	}
}

func TestOtherCausesReleaseDespiteAnMTDeliveryInProgress(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	ue.SetPagedBearerForTest(5, nil)
	ue.PagingAnswered()

	req := inactivityRelease(ue)
	req.Cause = s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioConnectionWithUELost})

	deliverReleaseRequest(t, m, cc, req)

	if len(cc.sent) != 1 {
		t.Fatalf("UE Context Release Commands sent = %d, want 1: only user inactivity is conditional on pending MT traffic", len(cc.sent))
	}
}
