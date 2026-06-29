// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
)

func TestStartSecurityModeRejectsNoCommonIntegrity(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.SetKASMEForTest(make([]byte, 32))
	ue.UeNetCap = eps.UENetworkCapability{EEA: 0xff, EIA: 0x00}.Marshal()

	startSecurityMode(m, context.Background(), ue)

	if ue.SecuredForTest() {
		t.Fatal("UE secured despite no common integrity algorithm")
	}

	reject, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("expected Attach Reject, got: %v", err)
	}

	if reject.Cause != mme.EmmCauseUESecCapsMismatch {
		t.Fatalf("Attach Reject cause = %d, want %d", reject.Cause, mme.EmmCauseUESecCapsMismatch)
	}
}
