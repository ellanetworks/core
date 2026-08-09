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

func newPreparingHandover(t *testing.T) (*amf.AMF, *amf.UeContext, *amf.UeConn, *amf.UeConn) {
	t.Helper()

	amfInstance := amf.New(nil, nil, nil)

	ue := amf.NewUeContext()
	ue.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")
	ue.SetNHForTest(make([]uint8, 32))

	source := amf.NewUeConnForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-source"), 1, 1, zap.NewNop())
	target := amf.NewUeConnForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-target"), 2, 2, zap.NewNop())

	source.AMFForTest().AttachUeConn(ue, source)

	if err := amf.SetHandoverForTest(source, target); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	return amfInstance, ue, source, target
}

// TestHandoverPreparing checks the read-only stage predicate used to drop a
// duplicate HANDOVER REQUEST ACKNOWLEDGE before it advances the FSM.
func TestHandoverPreparing(t *testing.T) {
	amfInstance, ue, _, _ := newPreparingHandover(t)

	if !amfInstance.HandoverPreparing(ue) {
		t.Fatal("a handover at the preparing stage must report HandoverPreparing")
	}

	if !amfInstance.MarkHandoverPrepared(ue, nil) {
		t.Fatal("MarkHandoverPrepared")
	}

	if amfInstance.HandoverPreparing(ue) {
		t.Fatal("a handover past the preparing stage must not report HandoverPreparing")
	}

	amfInstance.ClearHandover(ue)

	if amfInstance.HandoverPreparing(ue) {
		t.Fatal("no handover must not report HandoverPreparing")
	}
}

// TestCancelHandover verifies the gated cancel: a preparing/prepared handover is
// aborted (and a prepared target returned to release), while a committing handover
// is left intact for HANDOVER NOTIFY to finish — the race that would otherwise
// strand the UE on a released target (TS 38.413 §8.4.5).
func TestCancelHandover(t *testing.T) {
	t.Run("clears a prepared handover and returns the target to release", func(t *testing.T) {
		a, ue, source, target := newPreparingHandover(t)

		if !a.MarkHandoverPrepared(ue, nil) {
			t.Fatal("MarkHandoverPrepared")
		}

		got, aborted := a.CancelHandover(ue)
		if !aborted {
			t.Fatal("a prepared handover must be cancellable")
		}

		if got != target {
			t.Error("the prepared target must be returned for release")
		}

		if a.HandoverInProgress(ue) {
			t.Error("the FSM must be cleared after a cancel")
		}

		if ue.Conn() != source {
			t.Error("the UE must stay on the source after a cancel")
		}
	})

	t.Run("releases a preparing handover's reserved target", func(t *testing.T) {
		a, ue, source, target := newPreparingHandover(t)

		got, aborted := a.CancelHandover(ue)
		if !aborted {
			t.Fatal("a preparing handover must be cancellable")
		}

		// TS 38.413 §8.4.5: a still-preparing target's reserved resources must be
		// released on cancel, addressed by its AMF-UE-NGAP-ID (its RAN-UE-NGAP-ID has
		// not yet arrived); it is not left for a crossing acknowledge that may never come.
		if got != target {
			t.Error("a preparing handover must return its reserved target for release")
		}

		if a.HandoverInProgress(ue) {
			t.Error("the FSM must be cleared after a cancel")
		}

		if ue.Conn() != source {
			t.Error("the UE must stay on the source after a cancel")
		}
	})

	t.Run("does not cancel a committing handover", func(t *testing.T) {
		a, ue, _, target := newPreparingHandover(t)

		if !a.MarkHandoverPrepared(ue, nil) {
			t.Fatal("MarkHandoverPrepared")
		}

		if _, ok := a.MarkHandoverCommitting(ue, target); !ok {
			t.Fatal("MarkHandoverCommitting")
		}

		got, aborted := a.CancelHandover(ue)
		if aborted {
			t.Fatal("a committing handover must not be cancelled")
		}

		if got != nil {
			t.Error("a committing handover releases no target")
		}

		if !a.HandoverInProgress(ue) {
			t.Error("the committing FSM must be left intact for HANDOVER NOTIFY to finish")
		}
	})

	t.Run("no handover in progress acknowledges without action", func(t *testing.T) {
		a := amf.New(nil, nil, nil)
		ue := amf.NewUeContext()

		got, aborted := a.CancelHandover(ue)
		if aborted || got != nil {
			t.Fatal("no in-flight handover: nothing to cancel or release")
		}
	})
}

func TestHandoverGuardDoesNotAbandonCommitting(t *testing.T) {
	t.Run("abandons a prepared handover", func(t *testing.T) {
		a, ue, _, _ := newPreparingHandover(t)

		if !a.MarkHandoverPrepared(ue, nil) {
			t.Fatal("MarkHandoverPrepared")
		}

		if !a.FireHandoverGuardForTest(ue) {
			t.Fatal("the guard must abandon a handover that has not reached the committing stage")
		}

		if a.HandoverInProgress(ue) {
			t.Error("the FSM must be cleared after the guard fires")
		}
	})

	t.Run("leaves a committing handover to finish", func(t *testing.T) {
		a, ue, _, target := newPreparingHandover(t)

		if !a.MarkHandoverPrepared(ue, nil) {
			t.Fatal("MarkHandoverPrepared")
		}

		if _, ok := a.MarkHandoverCommitting(ue, target); !ok {
			t.Fatal("MarkHandoverCommitting")
		}

		if a.FireHandoverGuardForTest(ue) {
			t.Fatal("the guard must not abandon a committing handover")
		}

		if !a.HandoverInProgress(ue) {
			t.Fatal("the guard cleared a committing handover")
		}

		if !a.FinishHandoverCommit(ue, target) {
			t.Fatal("a committing handover must still finalize after the guard fires")
		}

		if ue.Conn() != target {
			t.Error("the UE must be on the target after finalize")
		}
	})
}

func TestFinishHandoverCommit(t *testing.T) {
	commit := func(t *testing.T, a *amf.AMF, ue *amf.UeContext, target *amf.UeConn) {
		t.Helper()

		if !a.MarkHandoverPrepared(ue, nil) {
			t.Fatal("MarkHandoverPrepared")
		}

		if _, ok := a.MarkHandoverCommitting(ue, target); !ok {
			t.Fatal("MarkHandoverCommitting")
		}
	}

	t.Run("moves the UE onto the target and clears the FSM", func(t *testing.T) {
		a, ue, _, target := newPreparingHandover(t)
		commit(t, a, ue, target)

		if !a.FinishHandoverCommit(ue, target) {
			t.Fatal("FinishHandoverCommit must succeed for a present target")
		}

		if ue.Conn() != target {
			t.Error("the UE must be moved onto the target UeConn")
		}

		if a.HandoverInProgress(ue) {
			t.Error("the FSM must be cleared after finalize")
		}
	})

	// TS 33.501 §6.9.2.3.3: the chain is advanced at preparation, so finalize must not
	// touch it, and a handover that fails to finalize leaves it advanced. Rolling back
	// would re-issue a pair already given to a gNB — see TestHandover_NHAdvancedAtPreparation.
	t.Run("finalize does not touch the key chain", func(t *testing.T) {
		a, ue, _, target := newPreparingHandover(t)

		nh, ncc := ue.NHForTest(), ue.NCCForTest()

		commit(t, a, ue, target)

		if !a.FinishHandoverCommit(ue, target) {
			t.Fatal("FinishHandoverCommit")
		}

		if ue.NHForTest() != nh || ue.NCCForTest() != ncc {
			t.Error("finalize must leave the chain as preparation set it")
		}
	})

	t.Run("a handover that fails to finalize leaves the chain advanced", func(t *testing.T) {
		a, ue, _, target := newPreparingHandover(t)

		nh, ncc := ue.NHForTest(), ue.NCCForTest()

		commit(t, a, ue, target)

		if err := a.RemoveUeConn(context.Background(), target); err != nil {
			t.Fatalf("RemoveUeConn: %v", err)
		}

		if a.FinishHandoverCommit(ue, target) {
			t.Fatal("FinishHandoverCommit must fail for a released target")
		}

		if ue.NHForTest() != nh || ue.NCCForTest() != ncc {
			t.Error("a failed finalize must not roll the chain back")
		}
	})

	t.Run("does not finalize before the committing stage", func(t *testing.T) {
		a, ue, source, target := newPreparingHandover(t)

		if !a.MarkHandoverPrepared(ue, nil) {
			t.Fatal("MarkHandoverPrepared")
		}

		if a.FinishHandoverCommit(ue, target) {
			t.Fatal("FinishHandoverCommit must fail before the committing stage")
		}

		if ue.Conn() != source {
			t.Error("the UE must stay on the source before finalize")
		}
	})

	t.Run("does not finalize onto a target released during the switch", func(t *testing.T) {
		a, ue, _, target := newPreparingHandover(t)
		commit(t, a, ue, target)

		if err := a.RemoveUeConn(context.Background(), target); err != nil {
			t.Fatalf("Remove: %v", err)
		}

		if a.FinishHandoverCommit(ue, target) {
			t.Fatal("FinishHandoverCommit must fail for a target released during the switch")
		}

		if ue.Conn() == target {
			t.Error("the UE must not be moved onto a released target")
		}
	})
}
