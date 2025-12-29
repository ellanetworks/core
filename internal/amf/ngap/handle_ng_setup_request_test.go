// Copyright 2025 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBDoesntSupportAnyTAC(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amfContext.AmfRan{
		Log:             logger.AmfLog,
		NGAPSender:      fakeNGAPSender,
		SupportedTAList: make([]amfContext.SupportedTAI, 0),
	}

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	amf := &amfContext.AMFContext{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc: "001",
				Mnc: "01",
				Sst: 1,
			},
		},
	}

	ngap.HandleNGSetupRequest(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure to be sent, but got %d", len(fakeNGAPSender.SentNGSetupFailures))
	}

	cause := fakeNGAPSender.SentNGSetupFailures[0].Cause
	if cause.Present != ngapType.CausePresentMisc {
		t.Fatalf("expected Cause Present to be Miscellaneous, but got %v", cause.Present)
	}

	if cause.Misc.Value != ngapType.CauseMiscPresentUnspecified {
		t.Errorf("expected Cause Miscellaneous Value to be CauseMiscPresentUnspecified, but got %v", cause.Misc.Value)
	}
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBSupportsDifferentTAC(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amfContext.AmfRan{
		Log:             logger.AmfLog,
		NGAPSender:      fakeNGAPSender,
		SupportedTAList: make([]amfContext.SupportedTAI, 0),
	}

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{
		Mcc: "001",
		Mnc: "01",
		Sst: 1,
	}

	err = op.SetSupportedTacs([]string{"000065", "000066"})
	if err != nil {
		t.Fatalf("failed to set supported TACS: %v", err)
	}

	amf := &amfContext.AMFContext{
		DBInstance: &FakeDBInstance{
			Operator: op,
		},
	}

	ngap.HandleNGSetupRequest(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure to be sent, but got %d", len(fakeNGAPSender.SentNGSetupFailures))
	}

	cause := fakeNGAPSender.SentNGSetupFailures[0].Cause
	if cause.Present != ngapType.CausePresentMisc {
		t.Fatalf("expected Cause Present to be Miscellaneous, but got %v", cause.Present)
	}

	if cause.Misc.Value != ngapType.CauseMiscPresentUnknownPLMN {
		t.Errorf("expected Cause Miscellaneous Value to be CauseMiscPresentUnknownPLMN, but got %v", cause.Misc.Value)
	}
}

func TestHandleNGSetupRequest_NGSetupResponse(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amfContext.AmfRan{
		Log:             logger.AmfLog,
		NGAPSender:      fakeNGAPSender,
		SupportedTAList: make([]amfContext.SupportedTAI, 0),
	}

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{
		Mcc: "001",
		Mnc: "01",
		Sst: 1,
	}

	err = op.SetSupportedTacs([]string{"000064", "000065"})
	if err != nil {
		t.Fatalf("failed to set supported TACS: %v", err)
	}

	amf := &amfContext.AMFContext{
		Name:             "ella-core",
		RelativeCapacity: 0xff,
		DBInstance: &FakeDBInstance{
			Operator: op,
		},
	}

	ngap.HandleNGSetupRequest(context.Background(), amf, ran, msg)

	if len(fakeNGAPSender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse to be sent, but got %d", len(fakeNGAPSender.SentNGSetupResponses))
	}

	response := fakeNGAPSender.SentNGSetupResponses[0]

	if response.Guami == nil {
		t.Errorf("expected Guami to be set in NGSetupResponse, but it was nil")
	}

	if response.Guami.PlmnID.Mcc != "001" {
		t.Errorf("expected Guami PlmnID MCC to be '001', but got %s", response.Guami.PlmnID.Mcc)
	}

	if response.Guami.PlmnID.Mnc != "01" {
		t.Errorf("expected Guami PlmnID MNC to be '01', but got %s", response.Guami.PlmnID.Mnc)
	}

	if response.PlmnSupported == nil {
		t.Errorf("expected PlmnSupported to be set in NGSetupResponse, but it was nil")
	}

	if response.PlmnSupported.PlmnID.Mcc != "001" {
		t.Errorf("expected PlmnSupported PlmnID MCC to be '001', but got %s", response.PlmnSupported.PlmnID.Mcc)
	}

	if response.PlmnSupported.PlmnID.Mnc != "01" {
		t.Errorf("expected PlmnSupported PlmnID MNC to be '01', but got %s", response.PlmnSupported.PlmnID.Mnc)
	}

	if response.AmfName != "ella-core" {
		t.Errorf("expected AmfName to be 'ella-core', but got '%s'", response.AmfName)
	}

	if response.AmfRelativeCapacity != 0xff {
		t.Errorf("expected AmfRelativeCapacity to be 0xff, but got %d", response.AmfRelativeCapacity)
	}
}
