// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
)

func newTestRadioForUeConn() *amf.Radio {
	ran := &amf.Radio{
		Conn: &fakeNGAPSender{},
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	return ran
}

func newBoundUeContext(t *testing.T, radio *amf.Radio) (*amf.UeContext, *amf.UeConn) {
	t.Helper()

	ueConn := amf.NewUeConnForTest(radio, 1, 10)

	ue := amf.NewUeContext()
	ueConn.AMFForTest().AttachUeConn(t.Context(), ue, ueConn)

	return ue, ueConn
}

func TestAttachUeConn_ReleasesDisplacedConn(t *testing.T) {
	radio := newTestRadioForUeConn()

	ue := amf.NewUeContext()

	first := amf.NewUeConnForTest(radio, 1, 10)
	amfInstance := first.AMFForTest()
	amfInstance.AttachUeConn(t.Context(), ue, first)

	second := amf.NewUeConnForTest(radio, 2, 11)
	amfInstance.AttachUeConn(t.Context(), ue, second)

	if first.UeContext() != nil {
		t.Fatal("displaced UeConn was not detached from the UE after re-attach")
	}

	if amfInstance.LookupUeConn(10) != first {
		t.Fatal("displaced UeConn must stay registered (ID reserved) until its Release Complete")
	}

	if amfInstance.LookupUeConn(11) != second {
		t.Fatal("new UeConn is not the UE's active connection after re-attach")
	}

	first.StopReleaseGuard()
	amfInstance.ReleaseUeConn(context.Background(), first)

	if amfInstance.LookupUeConn(10) != nil {
		t.Fatal("displaced UeConn was not reaped after its Release Complete (registry + NGAP-ID leak)")
	}
}

func TestReleaseNasConnection_AbortsProcedures(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, ueConn := newBoundUeContext(t, radio)

	if err := ue.Procedures().Begin(procedure.SecurityMode); err != nil {
		t.Fatalf("begin Authentication: %v", err)
	}

	ueConn.AMFForTest().ReleaseNasConnection(t.Context(), ue, ueConn)

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

	ueConn.AMFForTest().ReleaseNasConnection(t.Context(), ue, ueConn)

	waitFor(t, func() bool {
		return !ue.Procedures().Active(procedure.SecurityMode)
	})
}

func TestReleaseNasConnection_AfterRebind_IsNoop(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, sourceUeConn := newBoundUeContext(t, radio)

	targetUeConn := amf.NewUeConnForTest(radio, 2, 20)

	if err := ue.Procedures().Begin(procedure.N2Handover); err != nil {
		t.Fatalf("begin N2Handover: %v", err)
	}

	targetUeConn.AMFForTest().AttachUeConn(t.Context(), ue, targetUeConn)

	sourceUeConn.AMFForTest().ReleaseNasConnection(t.Context(), ue, sourceUeConn)

	if !ue.Procedures().Active(procedure.N2Handover) {
		t.Error("N2Handover aborted by stale source release")
	}

	if ue.Conn() != targetUeConn {
		t.Error("target UeConn was detached by stale source release")
	}
}

func TestReleaseNasConnection_StaleTarget_NoDetach(t *testing.T) {
	radio := newTestRadioForUeConn()
	ue, _ := newBoundUeContext(t, radio)

	staleUeConn := amf.NewUeConnForTest(radio, 99, 990)

	staleUeConn.AMFForTest().ReleaseNasConnection(t.Context(), ue, staleUeConn)

	if ue.Conn() == nil {
		t.Error("current UeConn was detached by stale release")
	}
}

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
	amf.NewUeConnForTest(radio, 1, 10)

	radio.AMFForTest().RemoveAllUeInRan(context.Background(), radio)

	if radio.NumUEsForTest() != 0 {
		t.Errorf("RanUEs count = %d, want 0", radio.NumUEsForTest())
	}
}

// TS 24.501 §5.5.1.3
func TestReleaseUeConn_HandoverKeepsTheContextOfANonRegisteredUE(t *testing.T) {
	for _, state := range []amf.StateType{
		amf.Deregistered,
		amf.RegistrationInitiated,
		amf.Registered,
		amf.DeregistrationInitiated,
	} {
		t.Run(state.String(), func(t *testing.T) {
			radio := newTestRadioForUeConn()
			ue, ueConn := newBoundUeContext(t, radio)
			ue.ForceStateForTest(state)

			supi := mustSUPI(t)
			ue.SetSupiForTest(supi)

			amfInstance := radio.AMFForTest()
			if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
				t.Fatalf("CommitUEIdentity: %v", err)
			}

			ueConn.ReleaseAction = amf.UeContextReleaseHandover
			amfInstance.ReleaseUeConn(context.Background(), ueConn)

			if _, ok := amfInstance.LookupUeBySupi(supi); !ok {
				t.Error("a handover release deleted the UE context")
			}
		})
	}
}

func TestDeregister_EndsKeyChainProcedures(t *testing.T) {
	for _, proc := range []procedure.Type{procedure.SecurityMode, procedure.N2Handover, procedure.PathSwitch} {
		t.Run(string(proc), func(t *testing.T) {
			radio := newTestRadioForUeConn()
			ue, _ := newBoundUeContext(t, radio)

			if err := ue.Procedures().Begin(proc); err != nil {
				t.Fatalf("Begin: %v", err)
			}

			ue.Deregister(context.Background())

			if ue.Procedures().Active(proc) {
				t.Error("a deregistered UE still holds its key chain")
			}
		})
	}
}

func TestReleaseUeConnServedBy_ReportsWhetherTheUEWentIdle(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    amf.StateType
		action   amf.RelAction
		wantIdle bool
	}{
		{"normal release of a registered UE", amf.Registered, amf.UeContextN2NormalRelease, true},
		{"normal release of a deregistered UE", amf.Deregistered, amf.UeContextN2NormalRelease, false},
		{"network-initiated deregistration", amf.Registered, amf.UeContextReleaseDueToNwInitiatedDeregistraion, false},
		{"aborted registration", amf.RegistrationInitiated, amf.UeContextReleaseAbortRegistration, false},
		{"handover", amf.Registered, amf.UeContextReleaseHandover, false},
		{"move to EPS", amf.Registered, amf.UeContextReleaseToEPS, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			radio := newTestRadioForUeConn()
			ue, ueConn := newBoundUeContext(t, radio)
			ue.ForceStateForTest(tc.state)
			ue.SetSupiForTest(mustSUPI(t))

			ueConn.ReleaseAction = tc.action

			if got := radio.AMFForTest().ReleaseUeConnServedBy(context.Background(), ueConn, nil); got != tc.wantIdle {
				t.Errorf("wentIdle = %v, want %v", got, tc.wantIdle)
			}
		})
	}
}
