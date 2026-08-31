// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func ranConfigUpdateWithTA(t *testing.T, mcc, mnc string, tac uint32) *ngap.RANConfigurationUpdate {
	t.Helper()

	plmn, err := util.PLMNToNGAP(models.PlmnID{Mcc: mcc, Mnc: mnc})
	if err != nil {
		t.Fatalf("could not encode PLMN: %v", err)
	}

	snssai, err := util.SNSSAIToNGAP(models.Snssai{Sst: 1})
	if err != nil {
		t.Fatalf("could not encode slice: %v", err)
	}

	return &ngap.RANConfigurationUpdate{
		SupportedTAList: ngap.SupportedTAList{{
			TAC: ngap.TAC(tac),
			BroadcastPLMNList: ngap.BroadcastPLMNList{{
				PLMNIdentity:        plmn,
				TAISliceSupportList: ngap.SliceSupportList{{SNSSAI: snssai}},
			}},
		}},
	}
}

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

// TS 38.413 §8.7.2.2
func TestHandleRANConfigurationUpdate_AbsentTAListPreservesAndAcks(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	existing := []amf.SupportedTAI{{Tai: models.Tai{Tac: "000064", PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}}}}
	amfInstance.UpdateRadioSupportedTAIs(ran, existing)
	amfInstance.UpdateRadioName(ran, "gNB-old")

	HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		&ngap.RANConfigurationUpdate{RANNodeName: ngap.Ptr("gNB-new")})

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

func TestHandleRANConfigurationUpdate_RejectPreservesTAs(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := operatorAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	stored := []amf.SupportedTAI{{Tai: models.Tai{Tac: "000064", PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}}}}
	amfInstance.UpdateRadioSupportedTAIs(ran, stored)

	HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
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

	HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
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

	HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		ranConfigUpdateWithTA(t, "001", "01", 0xFF))

	if len(sender.SentRanConfigurationUpdateAcks) != 0 {
		t.Fatalf("expected 0 acknowledges, got %d", len(sender.SentRanConfigurationUpdateAcks))
	}

	if len(sender.SentRanConfigurationUpdateFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	failure := sender.SentRanConfigurationUpdateFailures[0]
	want := ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnspecified}

	if failure.Cause == nil || *failure.Cause != want {
		t.Fatalf("cause = %v, want unspecified", failure.Cause)
	}
}

// TS 38.413 §8.7.2.3
func TestHandleRANConfigurationUpdate_NoMatchingPLMN(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := operatorAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		ranConfigUpdateWithTA(t, "999", "99", 0x64))

	if len(sender.SentRanConfigurationUpdateFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	failure := sender.SentRanConfigurationUpdateFailures[0]

	want := ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnknownPLMNOrSNPN}
	if failure.Cause == nil || *failure.Cause != want {
		t.Fatalf("cause = %v, want unknown-PLMN-or-SNPN", failure.Cause)
	}
}

// TS 38.413 §8.7.2.2
func TestHandleRANConfigurationUpdate_RebindsGlobalRANNodeID(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()

	newID := ngap.GlobalRANNodeID{
		Kind:         ngap.RANNodeIDGNB,
		PLMNIdentity: ngap.PLMNIdentity{0x00, 0xf1, 0x10},
		Value:        0x0000AB,
		Bits:         24,
	}

	HandleRANConfigurationUpdate(context.Background(), amfInstance, ran,
		&ngap.RANConfigurationUpdate{GlobalRANNodeID: &newID})

	if ran.RanID == nil {
		t.Fatal("Global RAN Node ID was not associated with the radio")
	}

	if found, ok := amfInstance.FindConnectedRadioByRanID(*ran.RanID); !ok || found != ran {
		t.Errorf("radio not reachable by its new Global RAN Node ID (ok=%v)", ok)
	}
}
