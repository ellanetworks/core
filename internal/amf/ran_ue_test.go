package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
)

func newTestRadioForRanUe() *amf.Radio {
	return &amf.Radio{
		Log:    logger.AmfLog,
		RanUEs: make(map[int64]*amf.RanUe),
	}
}

func newBoundAmfUe(t *testing.T, radio *amf.Radio) (*amf.AmfUe, *amf.RanUe) {
	t.Helper()

	ranUe := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewAmfUe()
	ue.Log = logger.AmfLog
	ue.AttachRanUe(ranUe)

	return ue, ranUe
}

// Per TS 24.501, ongoing NAS procedures shall be aborted on lower-layer
// failure.
func TestReleaseNasConnection_AbortsProcedures(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, ranUe := newBoundAmfUe(t, radio)

	conn := ue.NasConn()
	if _, err := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.Authentication}); err != nil {
		t.Fatalf("begin Authentication: %v", err)
	}

	ue.ReleaseNasConnection(ranUe)

	if ue.NasConn() != nil {
		t.Error("NAS connection still attached after release")
	}

	waitFor(t, func() bool {
		return !conn.Procedures.Active(procedure.Authentication)
	})

	if ue.RanUe() != nil {
		t.Error("RanUe still attached after release")
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
	radio := newTestRadioForRanUe()
	ue, ranUe := newBoundAmfUe(t, radio)

	conn := ue.NasConn()
	if _, err := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.SecurityMode}); err != nil {
		t.Fatalf("begin SecurityMode: %v", err)
	}

	ue.ReleaseNasConnection(ranUe)

	waitFor(t, func() bool {
		return !conn.Procedures.Active(procedure.SecurityMode)
	})
}

// After the AMF UE is rebound to a new RanUe, a release for the old
// RanUe (handover or any stale path) must be a no-op for both the
// procedure set and the current binding.
func TestReleaseNasConnection_AfterRebind_IsNoop(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, sourceRanUe := newBoundAmfUe(t, radio)

	targetRanUe := amf.NewRanUeForTest(radio, 2, 20, logger.AmfLog)

	conn := ue.NasConn()
	if _, err := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.N2Handover}); err != nil {
		t.Fatalf("begin N2Handover: %v", err)
	}

	ue.AttachRanUe(targetRanUe)

	ue.ReleaseNasConnection(sourceRanUe)

	if !conn.Procedures.Active(procedure.N2Handover) {
		t.Error("N2Handover aborted by stale source release")
	}

	if ue.RanUe() != targetRanUe {
		t.Error("target RanUe was detached by stale source release")
	}
}

// Verifies the target-match guard: a release for a stale RanUe must not
// detach the current one.
func TestReleaseNasConnection_StaleTarget_NoDetach(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, _ := newBoundAmfUe(t, radio)

	staleRanUe := amf.NewRanUeForTest(radio, 99, 990, logger.AmfLog)

	ue.ReleaseNasConnection(staleRanUe)

	if ue.RanUe() == nil {
		t.Error("current RanUe was detached by stale release")
	}
}

// SCTP disconnect aborts procedures across all UEs on the radio.
func TestRemoveAllUeInRan_AbortsProcedures(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, _ := newBoundAmfUe(t, radio)

	conn := ue.NasConn()
	if _, err := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.SecurityMode}); err != nil {
		t.Fatalf("begin SecurityMode: %v", err)
	}

	radio.RemoveAllUeInRan(context.Background())

	waitFor(t, func() bool {
		return !conn.Procedures.Active(procedure.SecurityMode)
	})
}

// Mid-registration UEs are deregistered on lower-layer failure
// (TS 24.501 §5.5.1.2.8(a)).
func TestRemoveAllUeInRan_MidAuthentication_Deregisters(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, _ := newBoundAmfUe(t, radio)
	ue.ForceState(amf.Authentication)

	radio.RemoveAllUeInRan(context.Background())

	if ue.GetState() != amf.Deregistered {
		t.Errorf("state = %s, want Deregistered", ue.GetState())
	}
}

func TestRemoveAllUeInRan_MidSecurityMode_Deregisters(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, _ := newBoundAmfUe(t, radio)
	ue.ForceState(amf.SecurityMode)

	radio.RemoveAllUeInRan(context.Background())

	if ue.GetState() != amf.Deregistered {
		t.Errorf("state = %s, want Deregistered", ue.GetState())
	}
}

func TestRemoveAllUeInRan_MidContextSetup_Deregisters(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, _ := newBoundAmfUe(t, radio)
	ue.ForceState(amf.ContextSetup)

	radio.RemoveAllUeInRan(context.Background())

	if ue.GetState() != amf.Deregistered {
		t.Errorf("state = %s, want Deregistered", ue.GetState())
	}
}

// Registered UEs keep their state and start the mobile reachable timer
// (TS 24.501 §5.3.7).
func TestRemoveAllUeInRan_Registered_StaysRegistered(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, _ := newBoundAmfUe(t, radio)
	ue.Current().T3512Value = 1 * time.Second
	ue.ForceState(amf.Registered)

	radio.RemoveAllUeInRan(context.Background())

	if ue.GetState() != amf.Registered {
		t.Errorf("state = %s, want Registered (mobile reachable timer running)", ue.GetState())
	}
}

func TestRemoveAllUeInRan_Deregistered_NoAction(t *testing.T) {
	radio := newTestRadioForRanUe()
	ue, _ := newBoundAmfUe(t, radio)

	radio.RemoveAllUeInRan(context.Background())

	if ue.GetState() != amf.Deregistered {
		t.Errorf("state = %s, want Deregistered", ue.GetState())
	}
}

func TestRemoveAllUeInRan_NoAmfUe(t *testing.T) {
	radio := newTestRadioForRanUe()
	amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	radio.RemoveAllUeInRan(context.Background())

	if len(radio.RanUEs) != 0 {
		t.Errorf("RanUEs count = %d, want 0", len(radio.RanUEs))
	}
}
