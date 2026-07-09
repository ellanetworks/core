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
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xff, EIA: 0x00}.Marshal(), nil, mme.MintAuthProofForAttachRequest())

	startSecurityMode(context.Background(), m, ue)

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

// TestStartSecurityModeClaimsKeyChain asserts the security mode procedure claims
// the {NH,NCC} key chain while the SECURITY MODE COMMAND is in flight, so a
// concurrent S1 handover / Path Switch is refused (TS 33.501 §6.9.5.1, TS 33.401
// §7.2.8).
func TestStartSecurityModeClaimsKeyChain(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.SetKASMEForTest(make([]byte, 32))
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xff, EIA: 0xff}.Marshal(), nil, mme.MintAuthProofForAttachRequest())

	startSecurityMode(context.Background(), m, ue)

	if len(cc.sent) == 0 {
		t.Fatal("expected a Security Mode Command to be sent")
	}

	if _, _, _, ok := m.BeginPathSwitch(ue); ok {
		t.Fatal("Path Switch started while a Security Mode Command was in flight")
	}
}
