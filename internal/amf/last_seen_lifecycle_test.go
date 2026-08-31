// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/sctp"
	"go.uber.org/zap"
)

func TestLastSeenRadioSurvivesIdleAndDeregistration(t *testing.T) {
	const imsi = "001010000000021"

	amfInstance := amf.New(nil, nil, nil)
	radio := newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gnb-a")
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())

	supi := newSUPI(t, imsi)

	ue := addTestUE(t, amfInstance, imsi, func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.Registered)
		ue.SetSecuredForTest(false)
	})
	amfInstance.AttachUeConn(ue, ueConn)

	if snap, _, _, ok := amfInstance.LookupSubscriber(supi); !ok || !snap.Connected {
		t.Fatalf("connected UE: found=%v Connected=%v, want true/true", ok, snap.Connected)
	}

	amfInstance.ReleaseUeConn(context.Background(), ueConn)

	snap, _, _, ok := amfInstance.LookupSubscriber(supi)
	if !ok {
		t.Fatal("idle UE missing from LookupSubscriber")
	}

	if snap.Connected {
		t.Error("Connected = true for a UE with no NGAP UE association")
	}

	if seen, ok := amfInstance.LastSeen(imsi); !ok || seen.RadioName != "gnb-a" {
		t.Errorf("idle UE last-seen radio = %q (found %v), want gnb-a", seen.RadioName, ok)
	}

	amfInstance.DeregisterSubscriber(context.Background(), supi)

	if _, _, _, ok := amfInstance.LookupSubscriber(supi); ok {
		t.Fatal("deregistered UE still reported as registered")
	}

	if seen, ok := amfInstance.LastSeen(imsi); !ok || seen.RadioName != "gnb-a" {
		t.Errorf("deregistered UE last-seen radio = %q (found %v), want it retained as gnb-a", seen.RadioName, ok)
	}

	amfInstance.ForgetSubscriber(imsi)

	if _, ok := amfInstance.LastSeen(imsi); ok {
		t.Error("ForgetSubscriber left the retained record in place")
	}
}

func TestLastSeenRadioFollowsARename(t *testing.T) {
	const imsi = "001010000000022"

	amfInstance := amf.New(nil, nil, nil)
	conn := &sctp.SCTPConn{}
	radio := newRadioForTest(amfInstance, conn, "gnb-a")
	amfInstance.SetRadioForTest(conn, radio)
	amfInstance.ClaimRanID(radio, gnbGlobalRANNodeID(t, "ABCDE1"))

	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ue := addTestUE(t, amfInstance, imsi, func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.Registered)
	})
	amfInstance.AttachUeConn(ue, ueConn)

	amfInstance.UpdateRadioName(radio, "gnb-a-renamed")

	seen, ok := amfInstance.LastSeen(imsi)
	if !ok {
		t.Fatal("retained record missing after rename")
	}

	if seen.RadioName != "gnb-a-renamed" {
		t.Errorf("RadioName = %q, want the current name gnb-a-renamed", seen.RadioName)
	}
}

func TestLastSeenRadioFallsBackToTheCapturedName(t *testing.T) {
	const imsi = "001010000000023"

	amfInstance := amf.New(nil, nil, nil)
	conn := &sctp.SCTPConn{}
	radio := newRadioForTest(amfInstance, conn, "gnb-unclaimed")
	amfInstance.SetRadioForTest(conn, radio)

	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ue := addTestUE(t, amfInstance, imsi, func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.Registered)
	})
	amfInstance.AttachUeConn(ue, ueConn)

	seen, ok := amfInstance.LastSeen(imsi)
	if !ok {
		t.Fatal("retained record missing")
	}

	if seen.RadioID != "" {
		t.Errorf("RadioID = %q, want empty for a radio with no Global RAN Node ID", seen.RadioID)
	}

	if seen.RadioName != "gnb-unclaimed" {
		t.Errorf("RadioName = %q, want the captured name gnb-unclaimed", seen.RadioName)
	}
}
