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

// TestPathSwitchNH_CommitOnlyOnConfirmedSwitch verifies the staged AS key chain
// at path switch: AdvancePathSwitchNH derives {NH, NCC} without touching the live
// chain, and CommitPathSwitch advances it only when the UE is still present,
// atomically re-pointing the UE at the target radio (TS 33.501 §6.9.2.1.1).
func TestPathSwitchNH_CommitOnlyOnConfirmedSwitch(t *testing.T) {
	makeUE := func() (*amf.AMF, *amf.UeContext, *amf.UeConn) {
		amfInstance := amf.New(nil, nil, nil)

		ue := amf.NewUeContext()
		ue.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")
		ue.SetNHForTest(make([]uint8, 32))
		ue.SetNCCForTest(3)

		radio := newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-source")
		ueConn := amf.NewUeConnForTest(radio, 5, 10, zap.NewNop())
		ueConn.AMFForTest().AttachUeConn(ue, ueConn)

		return amfInstance, ue, ueConn
	}

	t.Run("AdvancePathSwitchNH stages without advancing the live chain", func(t *testing.T) {
		_, ue, _ := makeUE()
		nh0, ncc0 := ue.NHForTest(), ue.NCCForTest()

		staged, stagedNCC, err := ue.AdvancePathSwitchNH()
		if err != nil {
			t.Fatalf("AdvancePathSwitchNH: %v", err)
		}

		if staged == nh0 {
			t.Fatal("staged NH must differ from the current NH")
		}

		if stagedNCC != (ncc0+1)%8 {
			t.Fatalf("staged NCC = %d, want %d", stagedNCC, (ncc0+1)%8)
		}

		if ue.NHForTest() != nh0 || ue.NCCForTest() != ncc0 {
			t.Fatal("AdvancePathSwitchNH must not advance the live NH chain")
		}
	})

	t.Run("CommitPathSwitch advances the chain and re-points the UE", func(t *testing.T) {
		amfInstance, ue, ueConn := makeUE()
		target := newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-target")

		staged, stagedNCC, err := ue.AdvancePathSwitchNH()
		if err != nil {
			t.Fatalf("AdvancePathSwitchNH: %v", err)
		}

		if !amfInstance.CommitPathSwitch(ue, ueConn, target, 99, staged, stagedNCC) {
			t.Fatal("CommitPathSwitch returned false for a live UE")
		}

		if ue.NHForTest() != staged || ue.NCCForTest() != stagedNCC {
			t.Fatal("CommitPathSwitch must commit the staged NH chain")
		}

		if ueConn.RanUeNgapID != 99 {
			t.Errorf("RanUeNgapID = %d, want 99", ueConn.RanUeNgapID)
		}

		if ueConn.Radio() != target {
			t.Error("UeConn must be re-pointed at the target radio")
		}
	})

	t.Run("CommitPathSwitch on a released UE leaves the chain unadvanced", func(t *testing.T) {
		amfInstance, ue, ueConn := makeUE()
		nh0, ncc0 := ue.NHForTest(), ue.NCCForTest()
		target := newRadioForTest(amfInstance, &sctp.SCTPConn{}, "gNB-target")

		staged, stagedNCC, err := ue.AdvancePathSwitchNH()
		if err != nil {
			t.Fatalf("AdvancePathSwitchNH: %v", err)
		}

		if err := amfInstance.RemoveUeConn(context.Background(), ueConn); err != nil {
			t.Fatalf("Remove: %v", err)
		}

		if amfInstance.CommitPathSwitch(ue, ueConn, target, 99, staged, stagedNCC) {
			t.Fatal("CommitPathSwitch must return false for a UE released during the switch")
		}

		if ue.NHForTest() != nh0 || ue.NCCForTest() != ncc0 {
			t.Fatal("a released UE must leave the NH chain unadvanced")
		}
	})
}
