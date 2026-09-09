// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"sync"
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

	if snap, _, ok := amfInstance.LookupSubscriber(supi); !ok || !snap.Connected {
		t.Fatalf("connected UE: found=%v Connected=%v, want true/true", ok, snap.Connected)
	}

	amfInstance.ReleaseUeConn(context.Background(), ueConn)

	snap, _, ok := amfInstance.LookupSubscriber(supi)
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

	if _, _, ok := amfInstance.LookupSubscriber(supi); ok {
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
	amfInstance.ClaimRanID(radio, gnbGlobalRANNodeID(t, "ABCDE1"), amf.DefaultRelativeCapacity)

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

func TestLastSeenRadioFollowsAnXnPathSwitch(t *testing.T) {
	const imsi = "001010000000024"

	amfInstance := amf.New(nil, nil, nil)

	sourceConn := &sctp.SCTPConn{}
	source := newRadioForTest(amfInstance, sourceConn, "gnb-a")
	amfInstance.SetRadioForTest(sourceConn, source)
	amfInstance.ClaimRanID(source, gnbGlobalRANNodeID(t, "ABCDE1"), amf.DefaultRelativeCapacity)

	targetConn := &sctp.SCTPConn{}
	target := newRadioForTest(amfInstance, targetConn, "gnb-b")
	amfInstance.SetRadioForTest(targetConn, target)
	amfInstance.ClaimRanID(target, gnbGlobalRANNodeID(t, "ABCDE2"), amf.DefaultRelativeCapacity)

	ueConn := amf.NewUeConnForTest(source, 1, 1, zap.NewNop())
	ue := addTestUE(t, amfInstance, imsi, func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.Registered)
	})
	amfInstance.AttachUeConn(ue, ueConn)

	if !amfInstance.CommitPathSwitch(ue, ueConn, target, 2, [32]uint8{}, 0) {
		t.Fatal("CommitPathSwitch reported the UE released")
	}

	seen, ok := amfInstance.LastSeen(imsi)
	if !ok {
		t.Fatal("retained record missing after the path switch")
	}

	if seen.RadioName != "gnb-b" {
		t.Errorf("RadioName = %q, want the target gnb-b", seen.RadioName)
	}
}

func TestRegisteringUEIsReportedAsConnectedButNotRegistered(t *testing.T) {
	const imsi = "001010000000025"

	amfInstance := amf.New(nil, nil, nil)
	radio := newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gnb-a")
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())

	ue := addTestUE(t, amfInstance, imsi, func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.RegistrationInitiated)
	})
	amfInstance.AttachUeConn(ue, ueConn)

	cs, ok := amfInstance.ConnectedSubscribers()[imsi]
	if !ok {
		t.Fatal("a UE part-way through registration is missing from ConnectedSubscribers")
	}

	if cs.Registered {
		t.Error("Registered = true for a UE that has not completed registration")
	}

	if !cs.Connected {
		t.Error("Connected = false for a UE holding an NGAP UE association")
	}

	snap, _, ok := amfInstance.LookupSubscriber(newSUPI(t, imsi))
	if !ok {
		t.Fatal("a UE part-way through registration is missing from LookupSubscriber")
	}

	if snap.Registered || !snap.Connected {
		t.Errorf("snapshot Registered=%v Connected=%v, want false/true", snap.Registered, snap.Connected)
	}
}

func TestUeConnRadioConcurrentAccess(t *testing.T) {
	const (
		imsi  = "001010000000026"
		iters = 1500
	)

	amfInstance := amf.New(nil, nil, nil)

	sourceConn := &sctp.SCTPConn{}
	source := newRadioForTest(amfInstance, sourceConn, "gnb-a")
	amfInstance.SetRadioForTest(sourceConn, source)
	amfInstance.ClaimRanID(source, gnbGlobalRANNodeID(t, "ABCDE1"), amf.DefaultRelativeCapacity)

	targetConn := &sctp.SCTPConn{}
	target := newRadioForTest(amfInstance, targetConn, "gnb-b")
	amfInstance.SetRadioForTest(targetConn, target)
	amfInstance.ClaimRanID(target, gnbGlobalRANNodeID(t, "ABCDE2"), amf.DefaultRelativeCapacity)

	ueConn := amf.NewUeConnForTest(source, 1, 1, zap.NewNop())
	ue := addTestUE(t, amfInstance, imsi, func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.Registered)
	})
	amfInstance.AttachUeConn(ue, ueConn)

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		for range iters {
			ueConn.TouchLastSeen()
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for range iters {
			amfInstance.CommitPathSwitch(ue, ueConn, target, 2, [32]uint8{}, 0)
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for range iters {
			_ = amfInstance.ConnectedSubscribers()
		}
	}()

	wg.Wait()

	amfInstance.CommitPathSwitch(ue, ueConn, target, 2, [32]uint8{}, 0)
	ueConn.TouchLastSeen()

	if seen, ok := amfInstance.LastSeen(imsi); !ok || seen.RadioName != "gnb-b" {
		t.Errorf("last-seen radio = %q (found %v), want gnb-b", seen.RadioName, ok)
	}
}

func TestLastSeenRadioIsRecordedWhenTheSupiArrivesAfterTheBind(t *testing.T) {
	const imsi = "001010000000027"

	amfInstance := amf.New(nil, nil, nil)
	radio := newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gnb-a")
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())

	ue := amf.NewUeContext()
	amfInstance.AttachUeConn(ue, ueConn)

	if _, ok := amfInstance.LastSeen(imsi); ok {
		t.Fatal("a record exists before the SUPI is known")
	}

	ue.SetSupi(newSUPI(t, imsi))

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	t.Cleanup(func() { amfInstance.DeregisterAndRemoveUeContext(context.Background(), ue) })

	seen, ok := amfInstance.LastSeen(imsi)
	if !ok {
		t.Fatal("no record after CommitUEIdentity; a UE registering with a SUCI is never captured")
	}

	if seen.RadioName != "gnb-a" {
		t.Errorf("RadioName = %q, want gnb-a", seen.RadioName)
	}
}

func TestDeregistrationInitiatedStillReportsRegistered(t *testing.T) {
	const imsi = "001010000000028"

	amfInstance := amf.New(nil, nil, nil)
	radio := newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gnb-a")
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())

	ue := addTestUE(t, amfInstance, imsi, func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.Registered)
	})
	amfInstance.AttachUeConn(ue, ueConn)

	ue.ForceStateForTest(amf.DeregistrationInitiated)

	cs, ok := amfInstance.ConnectedSubscribers()[imsi]
	if !ok {
		t.Fatal("a UE mid network-initiated deregistration is missing from ConnectedSubscribers")
	}

	if !cs.Registered {
		t.Error("Registered = false while the deregistration procedure is still in flight")
	}

	if !cs.Connected {
		t.Error("Connected = false for a UE still holding an NGAP UE association")
	}
}
