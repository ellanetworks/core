// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/free5gc/ngap/ngapType"
)

// ranConfigUpdateWithTA builds an update carrying one Supported TA Item for
// (mcc, mnc, tac), which is the shape the dispatcher hands the handler.
func ranConfigUpdateWithTA(t *testing.T, mcc, mnc string, tac uint32) *ngaplib.RANConfigurationUpdate {
	t.Helper()

	plmn, err := util.PLMNToNGAP(models.PlmnID{Mcc: mcc, Mnc: mnc})
	if err != nil {
		t.Fatalf("could not encode PLMN: %v", err)
	}

	snssai, err := util.SNSSAIToNGAP(models.Snssai{Sst: 1})
	if err != nil {
		t.Fatalf("could not encode slice: %v", err)
	}

	return &ngaplib.RANConfigurationUpdate{
		SupportedTAList: ngaplib.SupportedTAList{{
			TAC: ngaplib.TAC(tac),
			BroadcastPLMNList: ngaplib.BroadcastPLMNList{{
				PLMNIdentity:        plmn,
				TAISliceSupportList: ngaplib.SliceSupportList{{SNSSAI: snssai}},
			}},
		}},
	}
}

// operatorAMF is an AMF serving PLMN 001/01 and TAC 000064.
func operatorAMF() *amf.AMF {
	amfInstance := newTestAMFWithSmfAndDB(&fakeSmfSbi{})
	amfInstance.DBInstance = &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: `["000064"]`,
		},
	}

	return amfInstance
}

// TestHandleRANConfigurationUpdate_AbsentTAListPreservesAndAcks verifies that a
// name-only update (Supported TA List IE absent) is acknowledged, the stored TAs
// are left unchanged, and the RAN node name is applied (TS 38.413 §8.7.2.2).
func TestHandleRANConfigurationUpdate_AbsentTAListPreservesAndAcks(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	existing := []amf.SupportedTAI{{Tai: models.Tai{Tac: "000064", PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}}}}
	amfInstance.UpdateRadioSupportedTAIs(ran, existing)
	amfInstance.UpdateRadioName(ran, "gNB-old")

	ngap.HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		&ngaplib.RANConfigurationUpdate{RANNodeName: ngaplib.Ptr("gNB-new")})

	if len(sender.SentRanConfigurationUpdateFailures) != 0 {
		t.Fatalf("a name-only update must not fail, got %d failures", len(sender.SentRanConfigurationUpdateFailures))
	}

	if len(sender.SentRanConfigurationUpdateAcks) != 1 {
		t.Fatalf("expected 1 acknowledge, got %d", len(sender.SentRanConfigurationUpdateAcks))
	}

	if tais := amfInstance.RadioSupportedTAIsForTest(ran); len(tais) != 1 || tais[0].Tai.Tac != "000064" {
		t.Fatalf("absent Supported TA List must leave the stored TAs unchanged, got %+v", tais)
	}

	if name := amfInstance.RadioNameForTest(ran); name != "gNB-new" {
		t.Fatalf("RAN node name = %q, want gNB-new", name)
	}
}

// TestHandleRANConfigurationUpdate_RejectPreservesTAs verifies that an update
// whose Supported TA List names no served TAI is rejected without discarding
// the stored TAs.
func TestHandleRANConfigurationUpdate_RejectPreservesTAs(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := operatorAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	stored := []amf.SupportedTAI{{Tai: models.Tai{Tac: "000064", PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}}}}
	amfInstance.UpdateRadioSupportedTAIs(ran, stored)

	ngap.HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		ranConfigUpdateWithTA(t, "001", "01", 0xFF))

	if len(sender.SentRanConfigurationUpdateFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	if tais := amfInstance.RadioSupportedTAIsForTest(ran); len(tais) != 1 || tais[0].Tai.Tac != "000064" {
		t.Fatalf("a rejected update must not discard the stored TAs, got %+v", tais)
	}
}

func TestHandleRANConfigurationUpdate_MatchingTAs(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := operatorAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		ranConfigUpdateWithTA(t, "001", "01", 0x64))

	if len(sender.SentRanConfigurationUpdateAcks) != 1 {
		t.Fatalf("expected 1 acknowledge, got %d", len(sender.SentRanConfigurationUpdateAcks))
	}

	if len(sender.SentRanConfigurationUpdateFailures) != 0 {
		t.Fatalf("expected 0 failures, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	if tais := amfInstance.RadioSupportedTAIsForTest(ran); len(tais) != 1 || tais[0].Tai.Tac != "000064" {
		t.Fatalf("an accepted update must commit the new TAs, got %+v", tais)
	}
}

func TestHandleRANConfigurationUpdate_NoMatchingTAC(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := operatorAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		ranConfigUpdateWithTA(t, "001", "01", 0xFF))

	if len(sender.SentRanConfigurationUpdateAcks) != 0 {
		t.Fatalf("expected 0 acknowledges, got %d", len(sender.SentRanConfigurationUpdateAcks))
	}

	if len(sender.SentRanConfigurationUpdateFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	failure := sender.SentRanConfigurationUpdateFailures[0]
	if failure.Cause.Present != ngapType.CausePresentMisc {
		t.Fatalf("expected Misc cause, got present=%d", failure.Cause.Present)
	}

	// The gNB broadcasts a served PLMN but no served TAC; TS 38.413 has no
	// dedicated cause for an unserved TAC, so the reject cause is Misc/unspecified
	// (Unknown PLMN is reserved for when no PLMN matches).
	if failure.Cause.Misc == nil || failure.Cause.Misc.Value != ngapType.CauseMiscPresentUnspecified {
		t.Fatalf("expected Misc/Unspecified cause, got %+v", failure.Cause.Misc)
	}
}

// TestHandleRANConfigurationUpdate_NoMatchingPLMN rejects with Misc/Unknown PLMN
// when no PLMN the gNB broadcasts is served by the AMF (TS 38.413 §8.7.2.3).
func TestHandleRANConfigurationUpdate_NoMatchingPLMN(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := operatorAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		ranConfigUpdateWithTA(t, "999", "99", 0x64))

	if len(sender.SentRanConfigurationUpdateFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	failure := sender.SentRanConfigurationUpdateFailures[0]
	if failure.Cause.Misc == nil || failure.Cause.Misc.Value != ngapType.CauseMiscPresentUnknownPLMN {
		t.Fatalf("expected Misc/UnknownPLMN cause, got %+v", failure.Cause.Misc)
	}
}

// TS 38.413 §8.7.2.2: "If the Global RAN Node ID IE is included ... the AMF
// shall associate the TNLA to the NG-C interface instance using the Global RAN
// Node ID." §8.7.2.1 adds that the procedure "does not affect existing
// UE-related contexts", so the re-key must not disturb them.
func TestHandleRANConfigurationUpdate_RebindsGlobalRANNodeID(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	newID := ngaplib.GlobalRANNodeID{
		Kind:         ngaplib.RANNodeIDGNB,
		PLMNIdentity: ngaplib.PLMNIdentity{0x00, 0xf1, 0x10},
		Value:        0x0000AB,
		Bits:         24,
	}

	ngap.HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		&ngaplib.RANConfigurationUpdate{GlobalRANNodeID: &newID})

	if ran.RanID == nil {
		t.Fatal("Global RAN Node ID was not associated with the radio")
	}

	if found, ok := amfInstance.FindRadioByRanID(*ran.RanID); !ok || found != ran {
		t.Errorf("radio not reachable by its new Global RAN Node ID (ok=%v)", ok)
	}
}
