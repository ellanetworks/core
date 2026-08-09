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

func TestHandoverFSM_Lifecycle(t *testing.T) {
	amfUe := amf.NewUeContext()

	amfInstance := amf.New(nil, nil, nil)

	sourceUe := amf.NewUeConnForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-source"), 1, 1, zap.NewNop())
	targetUe := amf.NewUeConnForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-target"), 2, 2, zap.NewNop())

	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	if amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("a handover FSM exists before SetHandoverForTest")
	}

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	if !amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("handover FSM not installed by SetHandoverForTest")
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

func TestHandover_TargetRemovalAbortsHandover(t *testing.T) {
	amfUe := amf.NewUeContext()

	amfInstance := amf.New(nil, nil, nil)

	sourceUe := amf.NewUeConnForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-source"), 1, 1, zap.NewNop())
	targetUe := amf.NewUeConnForTest(newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-target"), 2, 2, zap.NewNop())

	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	if err := amf.SetHandoverForTest(sourceUe, targetUe); err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	if !amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("handover should be in progress")
	}

	if err := amfInstance.RemoveUeConn(context.Background(), targetUe); err != nil {
		t.Fatalf("Remove target: %v", err)
	}

	if amfInstance.HandoverInProgress(amfUe) {
		t.Fatal("removing the prepared target must abort the handover")
	}
}

// TestHandover_NHAdvancedAtPreparation verifies the AS key chain advances when the
// handover is prepared, and stays advanced however the handover ends.
//
// TS 33.501 §6.9.2.3.3: "Upon reception of the NGAP HANDOVER REQUIRED message ... the
// source AMF shall increment its locally kept NCC value by one and compute a fresh NH."
// The increment is unconditional. The pair is handed to the target gNB in the HANDOVER
// REQUEST, so rolling it back on abandonment would let the next handover issue the same
// {NH, NCC} to a different gNB — two gNBs each able to derive the other's KgNB, which
// §6.9.2.1.1 NOTE 3 ("the AMF always computes a fresh {NH, NCC} pair") rules out.
func TestHandover_NHAdvancedAtPreparation(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	makeUE := func() *amf.UeContext {
		ue := amf.NewUeContext()
		ue.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")
		ue.SetNHForTest(make([]uint8, 32))
		ue.SetNCCForTest(3)

		return ue
	}

	t.Run("preparation advances the live chain", func(t *testing.T) {
		ue := makeUE()
		nh0, ncc0 := ue.NHForTest(), ue.NCCForTest()

		staged, stagedNCC, ok := amfInstance.StageHandoverForTest(ue)
		if !ok {
			t.Fatal("StageHandoverForTest failed")
		}

		if staged == nh0 {
			t.Fatal("the derived NH should differ from the previous one")
		}

		if stagedNCC != (ncc0+1)%8 {
			t.Fatalf("derived NCC = %d, want %d", stagedNCC, (ncc0+1)%8)
		}

		if ue.NHForTest() != staged || ue.NCCForTest() != stagedNCC {
			t.Fatal("the pair sent to the target must be the UE's live chain")
		}
	})

	t.Run("an abandoned handover leaves the chain advanced", func(t *testing.T) {
		ue := makeUE()

		staged, stagedNCC, ok := amfInstance.StageHandoverForTest(ue)
		if !ok {
			t.Fatal("StageHandoverForTest failed")
		}

		amfInstance.ClearHandover(ue)

		// Rolling back here would re-issue this pair to the next target gNB.
		if ue.NHForTest() != staged || ue.NCCForTest() != stagedNCC {
			t.Fatal("an abandoned handover must not roll the chain back")
		}
	})

	t.Run("a second handover derives a different pair", func(t *testing.T) {
		ue := makeUE()

		first, firstNCC, ok := amfInstance.StageHandoverForTest(ue)
		if !ok {
			t.Fatal("StageHandoverForTest failed")
		}

		amfInstance.ClearHandover(ue)

		second, secondNCC, ok := amfInstance.StageHandoverForTest(ue)
		if !ok {
			t.Fatal("StageHandoverForTest failed")
		}

		if second == first {
			t.Error("a second handover reissued the NH already given to the first target")
		}

		if secondNCC != (firstNCC+1)%8 {
			t.Errorf("second NCC = %d, want %d", secondNCC, (firstNCC+1)%8)
		}
	})
}
