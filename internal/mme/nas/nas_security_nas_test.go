// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §5.4.3.5
func TestStartSecurityModeRejectsNoCommonIntegrity(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.SetKASMEForTest(make([]byte, 32))
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xff, EIA: 0x00}, nil, mme.MintAuthProofForAttachRequest())

	if got := startSecurityMode(context.Background(), m, ue, ue.Conn(), freshKeys); got != securityModeNoCommonAlgorithm {
		t.Fatalf("outcome = %d, want the no-common-algorithm report", got)
	}

	if ue.SecuredForTest() {
		t.Fatal("UE secured despite no common integrity algorithm")
	}

	if cc.count() != 0 {
		t.Fatalf("the security mode procedure sent %d messages, want the triggering procedure to answer", cc.count())
	}
}

// TS 24.301 §5.5.1.2.5
func TestAttachWithNoCommonIntegrityIsRejected(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.ForceRegStepForTest(mme.RegStepAuthenticating)
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xff, EIA: 0x00}, nil, mme.MintAuthProofForAttachRequest())

	xres := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	ue.Conn().AuthVector = &udm.EPSAV{XRES: xres, KASME: make([]byte, 32)}

	resp, err := (&eps.AuthenticationResponse{RES: xres}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), resp)

	if cc.count() == 0 {
		t.Fatal("the MME answered nothing, so the UE waits out T3410 before re-attaching")
	}

	reject, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("expected Attach Reject, got: %v", err)
	}

	if reject.Cause != eps.EMMCauseUESecurityCapabilitiesMismatch {
		t.Fatalf("Attach Reject cause = %d, want %d", reject.Cause, eps.EMMCauseUESecurityCapabilitiesMismatch)
	}
}

// TS 33.501 §6.9.5.1, TS 33.401 §7.2.8
func TestStartSecurityModeClaimsKeyChain(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.SetKASMEForTest(make([]byte, 32))
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xff, EIA: 0xff}, nil, mme.MintAuthProofForAttachRequest())

	startSecurityMode(context.Background(), m, ue, ue.Conn(), freshKeys)

	if len(cc.sent) == 0 {
		t.Fatal("expected a Security Mode Command to be sent")
	}

	if _, _, _, ok := m.BeginPathSwitch(ue); ok {
		t.Fatal("Path Switch started while a Security Mode Command was in flight")
	}
}
