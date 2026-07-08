// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

// TestHandleRanConfigurationUpdate_AbsentTAListPreservesAndAcks verifies that a
// name-only update (Supported TA List IE absent) is acknowledged, the stored TAs
// are left unchanged, and the RAN node name is applied (TS 38.413 §8.7.2.2).
func TestHandleRanConfigurationUpdate_AbsentTAListPreservesAndAcks(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	existing := []amf.SupportedTAI{{Tai: models.Tai{Tac: "000064", PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}}}}
	amfInstance.UpdateRadioSupportedTAIs(ran, existing)
	amfInstance.UpdateRadioName(ran, "gNB-old")

	ngap.HandleRanConfigurationUpdate(context.Background(), amfInstance, ran, decode.RANConfigurationUpdate{RANNodeName: "gNB-new"})

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

// TestHandleRanConfigurationUpdate_RejectPreservesTAs verifies that a present but
// unservable Supported TA List is rejected without discarding the stored TAs.
func TestHandleRanConfigurationUpdate_RejectPreservesTAs(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	amfInstance := newTestAMF()
	sender := ran.Conn.(*fakeNGAPSender)

	amfInstance.UpdateRadioSupportedTAIs(ran, []amf.SupportedTAI{{Tai: models.Tai{Tac: "000064", PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}}}})

	// Present-but-empty Supported TA List → no served TAI → Failure.
	ngap.HandleRanConfigurationUpdate(context.Background(), amfInstance, ran, decode.RANConfigurationUpdate{SupportedTAItems: []ngapType.SupportedTAItem{}})

	if len(sender.SentRanConfigurationUpdateFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	if tais := amfInstance.RadioSupportedTAIsForTest(ran); len(tais) != 1 {
		t.Fatalf("a rejected update must not discard the stored TAs, got %+v", tais)
	}
}

func TestHandleRanConfigurationUpdate_MatchingTAs(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)

	amfInstance := newTestAMFWithSmfAndDB(&fakeSmfSbi{})
	amfInstance.DBInstance = &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: `["000064"]`,
		},
	}

	plmnID, err := getMccAndMncInOctets("001", "01")
	if err != nil {
		t.Fatal(err)
	}

	sst, _, err := getSliceInBytes(1, "")
	if err != nil {
		t.Fatal(err)
	}

	msg := decode.RANConfigurationUpdate{
		SupportedTAItems: []ngapType.SupportedTAItem{
			{
				TAC: ngapType.TAC{Value: []byte{0x00, 0x00, 0x64}},
				BroadcastPLMNList: ngapType.BroadcastPLMNList{
					List: []ngapType.BroadcastPLMNItem{
						{
							PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
							TAISliceSupportList: ngapType.SliceSupportList{
								List: []ngapType.SliceSupportItem{
									{SNSSAI: ngapType.SNSSAI{SST: ngapType.SST{Value: sst}}},
								},
							},
						},
					},
				},
			},
		},
	}

	ngap.HandleRanConfigurationUpdate(context.Background(), amfInstance, ran, msg)

	if len(sender.SentRanConfigurationUpdateAcks) != 1 {
		t.Fatalf("expected 1 acknowledge, got %d", len(sender.SentRanConfigurationUpdateAcks))
	}

	if len(sender.SentRanConfigurationUpdateFailures) != 0 {
		t.Fatalf("expected 0 failures, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}
}

func TestHandleRanConfigurationUpdate_NoMatchingTAC(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)

	amfInstance := newTestAMFWithSmfAndDB(&fakeSmfSbi{})
	amfInstance.DBInstance = &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: `["000064"]`,
		},
	}

	plmnID, err := getMccAndMncInOctets("001", "01")
	if err != nil {
		t.Fatal(err)
	}

	sst, _, err := getSliceInBytes(1, "")
	if err != nil {
		t.Fatal(err)
	}

	msg := decode.RANConfigurationUpdate{
		SupportedTAItems: []ngapType.SupportedTAItem{
			{
				TAC: ngapType.TAC{Value: []byte{0x00, 0x00, 0xFF}},
				BroadcastPLMNList: ngapType.BroadcastPLMNList{
					List: []ngapType.BroadcastPLMNItem{
						{
							PLMNIdentity: ngapType.PLMNIdentity{Value: plmnID},
							TAISliceSupportList: ngapType.SliceSupportList{
								List: []ngapType.SliceSupportItem{
									{SNSSAI: ngapType.SNSSAI{SST: ngapType.SST{Value: sst}}},
								},
							},
						},
					},
				},
			},
		},
	}

	ngap.HandleRanConfigurationUpdate(context.Background(), amfInstance, ran, msg)

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

// TestHandleRanConfigurationUpdate_NoMatchingPLMN rejects with Misc/Unknown PLMN
// when no PLMN the gNB broadcasts is served by the AMF (TS 38.413 §8.7.1.4).
func TestHandleRanConfigurationUpdate_NoMatchingPLMN(t *testing.T) {
	ran := newTestRadio(newTestAMF())
	sender := ran.Conn.(*fakeNGAPSender)

	amfInstance := newTestAMFWithSmfAndDB(&fakeSmfSbi{})
	amfInstance.DBInstance = &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: `["000064"]`,
		},
	}

	otherPLMN, err := getMccAndMncInOctets("999", "99")
	if err != nil {
		t.Fatal(err)
	}

	sst, _, err := getSliceInBytes(1, "")
	if err != nil {
		t.Fatal(err)
	}

	msg := decode.RANConfigurationUpdate{
		SupportedTAItems: []ngapType.SupportedTAItem{
			{
				TAC: ngapType.TAC{Value: []byte{0x00, 0x00, 0x64}},
				BroadcastPLMNList: ngapType.BroadcastPLMNList{
					List: []ngapType.BroadcastPLMNItem{
						{
							PLMNIdentity: ngapType.PLMNIdentity{Value: otherPLMN},
							TAISliceSupportList: ngapType.SliceSupportList{
								List: []ngapType.SliceSupportItem{
									{SNSSAI: ngapType.SNSSAI{SST: ngapType.SST{Value: sst}}},
								},
							},
						},
					},
				},
			},
		},
	}

	ngap.HandleRanConfigurationUpdate(context.Background(), amfInstance, ran, msg)

	if len(sender.SentRanConfigurationUpdateFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	failure := sender.SentRanConfigurationUpdateFailures[0]
	if failure.Cause.Misc == nil || failure.Cause.Misc.Value != ngapType.CauseMiscPresentUnknownPLMN {
		t.Fatalf("expected Misc/UnknownPLMN cause, got %+v", failure.Cause.Misc)
	}
}
