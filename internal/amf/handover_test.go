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

// TestHandoverFSM_Lifecycle asserts the handover FSM — the single source of truth
// for the source/target RanUe pair — is installed by AttachSourceUeTargetUe and
// torn down by ClearHandover.
func TestHandoverFSM_Lifecycle(t *testing.T) {
	amfUe := amf.NewUeContext()

	amfInstance := amf.New(nil, nil, nil)

	sourceUe := amf.NewRanUeForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-source"), 1, 1, zap.NewNop())
	targetUe := amf.NewRanUeForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-target"), 2, 2, zap.NewNop())

	amfUe.AttachRanUe(sourceUe)

	if amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("a handover FSM exists before AttachSourceUeTargetUe")
	}

	if err := amf.AttachSourceUeTargetUe(sourceUe, targetUe); err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	if !amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("handover FSM not installed by AttachSourceUeTargetUe")
	}

	if amfInstance.HandoverSource(amfUe) != sourceUe {
		t.Errorf("HandoverSource = %p, want source %p", amfInstance.HandoverSource(amfUe), sourceUe)
	}

	if amfInstance.HandoverTarget(amfUe) != targetUe {
		t.Errorf("HandoverTarget = %p, want target %p", amfInstance.HandoverTarget(amfUe), targetUe)
	}

	amfInstance.ClearHandover(amfUe)

	if amfInstance.HandoverInProgress(amfUe) || amfInstance.HandoverSource(amfUe) != nil || amfInstance.HandoverTarget(amfUe) != nil {
		t.Error("handover FSM not cleared by ClearHandover")
	}
}

// TestHandover_TargetRemovalAbortsHandover verifies that removing the prepared
// target RanUe (its gNB association reset or lost) clears the in-flight handover at
// once, rather than leaving it dangling until the supervision guard fires.
func TestHandover_TargetRemovalAbortsHandover(t *testing.T) {
	amfUe := amf.NewUeContext()

	amfInstance := amf.New(nil, nil, nil)

	sourceUe := amf.NewRanUeForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-source"), 1, 1, zap.NewNop())
	targetUe := amf.NewRanUeForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-target"), 2, 2, zap.NewNop())

	amfUe.AttachRanUe(sourceUe)

	if err := amf.AttachSourceUeTargetUe(sourceUe, targetUe); err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	if !amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("handover should be in progress")
	}

	if err := targetUe.Remove(context.Background()); err != nil {
		t.Fatalf("Remove target: %v", err)
	}

	if amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("removing the prepared target must abort the handover")
	}
}

// TestHandover_NHCommittedOnlyOnCompletion verifies the staged AS key chain is
// committed to the UE only when the handover completes (HANDOVER NOTIFY), and an
// abandoned handover leaves the live {NH, NCC} untouched (TS 33.501 §6.9.2.1.1).
func TestHandover_NHCommittedOnlyOnCompletion(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	makeUE := func() *amf.UeContext {
		ue := amf.NewUeContext()
		ue.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")
		ue.SetNHForTest(make([]uint8, 32))
		ue.SetNCCForTest(3)

		return ue
	}

	t.Run("abandoned handover does not advance the live NH chain", func(t *testing.T) {
		ue := makeUE()
		nh0, ncc0 := ue.NHForTest(), ue.NCCForTest()

		if err := amfInstance.BeginHandover(ue, nil, nil); err != nil {
			t.Fatalf("BeginHandover: %v", err)
		}

		staged, stagedNCC, ok := amfInstance.StagedHandoverNH(ue)
		if !ok {
			t.Fatal("expected a staged NH")
		}

		if staged == nh0 {
			t.Fatal("staged NH should differ from the current NH")
		}

		if stagedNCC != (ncc0+1)%8 {
			t.Fatalf("staged NCC = %d, want %d", stagedNCC, (ncc0+1)%8)
		}

		amfInstance.ClearHandover(ue)

		if ue.NHForTest() != nh0 || ue.NCCForTest() != ncc0 {
			t.Fatal("abandoned handover must not advance the live NH chain")
		}
	})

	t.Run("completed handover commits the staged NH chain", func(t *testing.T) {
		ue := makeUE()

		if err := amfInstance.BeginHandover(ue, nil, nil); err != nil {
			t.Fatalf("BeginHandover: %v", err)
		}

		staged, stagedNCC, _ := amfInstance.StagedHandoverNH(ue)

		if !amfInstance.MarkHandoverPrepared(ue) {
			t.Fatal("MarkHandoverPrepared returned false")
		}

		if !amfInstance.MarkHandoverCommitting(ue) {
			t.Fatal("MarkHandoverCommitting returned false")
		}

		if ue.NHForTest() != staged || ue.NCCForTest() != stagedNCC {
			t.Fatal("completed handover must commit the staged NH chain")
		}
	})
}
