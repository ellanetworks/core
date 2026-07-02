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

func newPreparingHandover(t *testing.T) (*amf.AMF, *amf.UeContext, *amf.RanUe, *amf.RanUe) {
	t.Helper()

	amfInstance := amf.New(nil, nil, nil)

	ue := amf.NewUeContext()
	ue.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")
	ue.SetNHForTest(make([]uint8, 32))

	source := amf.NewRanUeForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-source"), 1, 1, zap.NewNop())
	target := amf.NewRanUeForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-target"), 2, 2, zap.NewNop())

	ue.AttachRanUe(source)

	if err := amf.AttachSourceUeTargetUe(source, target); err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
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

	if !amfInstance.MarkHandoverPrepared(ue) {
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

// TestFinishHandoverCommit verifies the gated finalize: the UE moves onto the
// target only from the committing stage and only while the target is still
// present after the (unlocked) user-plane switch (TS 23.502).
func TestFinishHandoverCommit(t *testing.T) {
	commit := func(t *testing.T, a *amf.AMF, ue *amf.UeContext) {
		t.Helper()

		if !a.MarkHandoverPrepared(ue) {
			t.Fatal("MarkHandoverPrepared")
		}

		if !a.MarkHandoverCommitting(ue) {
			t.Fatal("MarkHandoverCommitting")
		}
	}

	t.Run("moves the UE onto the target and clears the FSM", func(t *testing.T) {
		a, ue, _, target := newPreparingHandover(t)
		commit(t, a, ue)

		if !a.FinishHandoverCommit(ue, target) {
			t.Fatal("FinishHandoverCommit must succeed for a present target")
		}

		if ue.RanUe() != target {
			t.Error("the UE must be moved onto the target RanUe")
		}

		if a.HandoverInProgress(ue) {
			t.Error("the FSM must be cleared after finalize")
		}
	})

	t.Run("does not finalize before the committing stage", func(t *testing.T) {
		a, ue, source, target := newPreparingHandover(t)

		if !a.MarkHandoverPrepared(ue) {
			t.Fatal("MarkHandoverPrepared")
		}

		if a.FinishHandoverCommit(ue, target) {
			t.Fatal("FinishHandoverCommit must fail before the committing stage")
		}

		if ue.RanUe() != source {
			t.Error("the UE must stay on the source before finalize")
		}
	})

	t.Run("does not finalize onto a target released during the switch", func(t *testing.T) {
		a, ue, _, target := newPreparingHandover(t)
		commit(t, a, ue)

		if err := target.Remove(context.Background()); err != nil {
			t.Fatalf("Remove: %v", err)
		}

		if a.FinishHandoverCommit(ue, target) {
			t.Fatal("FinishHandoverCommit must fail for a target released during the switch")
		}

		if ue.RanUe() == target {
			t.Error("the UE must not be moved onto a released target")
		}
	})
}
