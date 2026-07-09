// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
)

func newTestRadioForUeConn() *amf.Radio {
	ran := &amf.Radio{
		Conn: &fakeNGAPSender{},
		Log:  logger.AmfLog,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	return ran
}

func newBoundUeContext(t *testing.T, radio *amf.Radio) (*amf.UeContext, *amf.UeConn) {
	t.Helper()

	ueConn := amf.NewUeConnForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewUeContext()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	return ue, ueConn
}

// A UE re-attaching on a new connection must release its previous connection, so the
// displaced UeConn + AMF-UE-NGAP-ID do not leak in the registry.
func TestAttachUeConn_ReleasesDisplacedConn(t *testing.T) {
	radio := newTestRadioForUeConn()

	ue := amf.NewUeContext()

	first := amf.NewUeConnForTest(radio, 1, 10, logger.AmfLog)
	amfInstance := first.AMFForTest()
	amfInstance.AttachUeConn(ue, first)

	second := amf.NewUeConnForTest(radio, 2, 11, logger.AmfLog)
	amfInstance.AttachUeConn(ue, second)

	if amfInstance.LookupUeConn(10) != nil {
		t.Fatal("displaced UeConn was not released after re-attach (registry + NGAP-ID leak)")
	}

	if amfInstance.LookupUeConn(11) != second {
		t.Fatal("new UeConn is not the UE's active connection after re-attach")
	}
}

// Per TS 24.501, ongoing NAS procedures shall be aborted on lower-layer
// failure.
func TestReleaseNasConnection_AbortsProcedures(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, ueConn := newBoundUeContext(t, radio)

	if err := ue.Procedures().Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("begin Authentication: %v", err)
	}

	ueConn.AMFForTest().ReleaseNasConnection(ue, ueConn)

	if ue.Conn() != nil {
		t.Error("NAS connection still attached after release")
	}

	waitFor(t, func() bool {
		return !ue.Procedures().Active(procedure.SecurityMode)
	})

	if ue.Conn() != nil {
		t.Error("UeConn still attached after release")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for !cond() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if !cond() {
		t.Fatal("condition not met within deadline")
	}
}

func TestReleaseNasConnection_AbortsSecurityMode(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, ueConn := newBoundUeContext(t, radio)

	if err := ue.Procedures().Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("begin SecurityMode: %v", err)
	}

	ueConn.AMFForTest().ReleaseNasConnection(ue, ueConn)

	waitFor(t, func() bool {
		return !ue.Procedures().Active(procedure.SecurityMode)
	})
}

// After the AMF UE is rebound to a new UeConn, a release for the old
// UeConn (handover or any stale path) must be a no-op for both the
// procedure set and the current binding.
func TestReleaseNasConnection_AfterRebind_IsNoop(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, sourceUeConn := newBoundUeContext(t, radio)

	targetUeConn := amf.NewUeConnForTest(radio, 2, 20, logger.AmfLog)

	if err := ue.Procedures().Begin(procedure.N2Handover); err != nil {
		t.Fatalf("begin N2Handover: %v", err)
	}

	targetUeConn.AMFForTest().AttachUeConn(ue, targetUeConn)

	sourceUeConn.AMFForTest().ReleaseNasConnection(ue, sourceUeConn)

	if !ue.Procedures().Active(procedure.N2Handover) {
		t.Error("N2Handover aborted by stale source release")
	}

	if ue.Conn() != targetUeConn {
		t.Error("target UeConn was detached by stale source release")
	}
}

// Verifies the target-match guard: a release for a stale UeConn must not
// detach the current one.
func TestReleaseNasConnection_StaleTarget_NoDetach(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, _ := newBoundUeContext(t, radio)

	staleUeConn := amf.NewUeConnForTest(radio, 99, 990, logger.AmfLog)

	staleUeConn.AMFForTest().ReleaseNasConnection(ue, staleUeConn)

	if ue.Conn() == nil {
		t.Error("current UeConn was detached by stale release")
	}
}

// SCTP disconnect aborts procedures across all UEs on the radio.
func TestRemoveAllUeInRan_AbortsProcedures(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, _ := newBoundUeContext(t, radio)

	if err := ue.Procedures().Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("begin SecurityMode: %v", err)
	}

	radio.AMFForTest().RemoveAllUeInRan(context.Background(), radio)

	waitFor(t, func() bool {
		return !ue.Procedures().Active(procedure.SecurityMode)
	})
}

// Mid-registration UEs are deregistered on lower-layer failure
// (TS 24.501).
func TestRemoveAllUeInRan_MidAuthentication_Deregisters(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, _ := newBoundUeContext(t, radio)
	ue.ForceRegStepForTest(amf.RegStepAuthenticating)

	radio.AMFForTest().RemoveAllUeInRan(context.Background(), radio)

	if ue.State() != amf.Deregistered {
		t.Errorf("state = %s, want Deregistered", ue.State())
	}
}

func TestRemoveAllUeInRan_MidSecurityMode_Deregisters(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, _ := newBoundUeContext(t, radio)
	ue.ForceRegStepForTest(amf.RegStepSecurityMode)

	radio.AMFForTest().RemoveAllUeInRan(context.Background(), radio)

	if ue.State() != amf.Deregistered {
		t.Errorf("state = %s, want Deregistered", ue.State())
	}
}

func TestRemoveAllUeInRan_MidContextSetup_Deregisters(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, _ := newBoundUeContext(t, radio)
	ue.ForceRegStepForTest(amf.RegStepContextSetup)

	radio.AMFForTest().RemoveAllUeInRan(context.Background(), radio)

	if ue.State() != amf.Deregistered {
		t.Errorf("state = %s, want Deregistered", ue.State())
	}
}

// Registered UEs keep their state and start the mobile reachable timer
// (TS 24.501).
func TestRemoveAllUeInRan_Registered_StaysRegistered(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, _ := newBoundUeContext(t, radio)
	ue.ForceStateForTest(amf.Registered)

	radio.AMFForTest().RemoveAllUeInRan(context.Background(), radio)

	if ue.State() != amf.Registered {
		t.Errorf("state = %s, want Registered (mobile reachable timer running)", ue.State())
	}
}

func TestRemoveAllUeInRan_Deregistered_NoAction(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, _ := newBoundUeContext(t, radio)

	radio.AMFForTest().RemoveAllUeInRan(context.Background(), radio)

	if ue.State() != amf.Deregistered {
		t.Errorf("state = %s, want Deregistered", ue.State())
	}
}

func TestRemoveAllUeInRan_NoUeContext(t *testing.T) {
	radio := newTestRadioForUeConn()
	amf.NewUeConnForTest(radio, 1, 10, logger.AmfLog)

	radio.AMFForTest().RemoveAllUeInRan(context.Background(), radio)

	if radio.NumUEsForTest() != 0 {
		t.Errorf("RanUEs count = %d, want 0", radio.NumUEsForTest())
	}
}
