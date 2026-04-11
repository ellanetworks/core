// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/db"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleRanConfigurationUpdate_NoSupportedTAs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	ngap.HandleRanConfigurationUpdate(context.Background(), amfInstance, ran, decode.RANConfigurationUpdate{})

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

	if failure.Cause.Misc == nil || failure.Cause.Misc.Value != ngapType.CauseMiscPresentUnspecified {
		t.Fatalf("expected Misc/Unspecified cause, got %+v", failure.Cause.Misc)
	}
}

func TestHandleRanConfigurationUpdate_MatchingTAs(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	amfInstance := newTestAMFWithSmfAndDB(&FakeSmfSbi{})
	amfInstance.DBInstance = &FakeDBInstance{
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
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	amfInstance := newTestAMFWithSmfAndDB(&FakeSmfSbi{})
	amfInstance.DBInstance = &FakeDBInstance{
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

	if failure.Cause.Misc == nil || failure.Cause.Misc.Value != ngapType.CauseMiscPresentUnknownPLMN {
		t.Fatalf("expected Misc/UnknownPLMN cause, got %+v", failure.Cause.Misc)
	}
}
