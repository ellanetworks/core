// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
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

	sourceUe := amf.NewRanUeForTest(newRadioForTest(&sctp.SCTPConn{}, "gNB-source"), 1, 1, zap.NewNop())
	targetUe := amf.NewRanUeForTest(newRadioForTest(&sctp.SCTPConn{}, "gNB-target"), 2, 2, zap.NewNop())

	amfUe.AttachRanUe(sourceUe)

	if amfUe.HandoverInProgress() {
		t.Fatal("a handover FSM exists before AttachSourceUeTargetUe")
	}

	if err := amf.AttachSourceUeTargetUe(sourceUe, targetUe); err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	if !amfUe.HandoverInProgress() {
		t.Fatal("handover FSM not installed by AttachSourceUeTargetUe")
	}

	if amfUe.HandoverSource() != sourceUe {
		t.Errorf("HandoverSource = %p, want source %p", amfUe.HandoverSource(), sourceUe)
	}

	if amfUe.HandoverTarget() != targetUe {
		t.Errorf("HandoverTarget = %p, want target %p", amfUe.HandoverTarget(), targetUe)
	}

	amfUe.ClearHandover()

	if amfUe.HandoverInProgress() || amfUe.HandoverSource() != nil || amfUe.HandoverTarget() != nil {
		t.Error("handover FSM not cleared by ClearHandover")
	}
}
