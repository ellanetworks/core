// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/free5gc/ngap/ngapType"
)

type SliceOpt struct {
	Sst int32
	Sd  string
}

type NGSetupRequestOpts struct {
	Name   string
	GnbID  string
	ID     int64
	Mcc    string
	Mnc    string
	Tac    string
	Sst    int32
	Sd     string
	Slices []SliceOpt // if set, overrides Sst/Sd
}

// buildNGSetupRequest assembles the request as the library models it, which is
// what the handler now receives from the dispatcher.
func buildNGSetupRequest(opts *NGSetupRequestOpts) (*ngaplib.NGSetupRequest, error) {
	if opts.Mcc == "" || opts.Mnc == "" {
		return nil, fmt.Errorf("MCC and MNC are required to build NGSetupRequest")
	}

	plmn, err := util.PLMNToNGAP(models.PlmnID{Mcc: opts.Mcc, Mnc: opts.Mnc})
	if err != nil {
		return nil, fmt.Errorf("could not encode PLMN: %w", err)
	}

	gnbID, err := strconv.ParseUint(opts.GnbID, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("could not parse gNB id %q: %w", opts.GnbID, err)
	}

	slices := opts.Slices
	if len(slices) == 0 {
		if opts.Sst == 0 {
			return nil, fmt.Errorf("SST is required to build NGSetupRequest")
		}

		slices = []SliceOpt{{Sst: opts.Sst, Sd: opts.Sd}}
	}

	req := &ngaplib.NGSetupRequest{
		GlobalRANNodeID: ngaplib.GlobalRANNodeID{
			Kind:         ngaplib.RANNodeIDGNB,
			PLMNIdentity: plmn,
			Value:        uint32(gnbID),
			Bits:         24,
		},
		RANNodeName:      ngaplib.Ptr(opts.Name),
		DefaultPagingDRX: ngaplib.Ptr(ngaplib.PagingDRXv128),
	}

	if opts.Tac == "" {
		return req, nil
	}

	tac, err := strconv.ParseUint(opts.Tac, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("could not parse TAC %q: %w", opts.Tac, err)
	}

	support := make(ngaplib.SliceSupportList, 0, len(slices))

	for _, sl := range slices {
		snssai, err := util.SNSSAIToNGAP(models.Snssai{Sst: sl.Sst, Sd: sl.Sd})
		if err != nil {
			return nil, fmt.Errorf("could not encode slice: %w", err)
		}

		support = append(support, ngaplib.SliceSupportItem{SNSSAI: snssai})
	}

	req.SupportedTAList = ngaplib.SupportedTAList{{
		TAC: ngaplib.TAC(tac),
		BroadcastPLMNList: ngaplib.BroadcastPLMNList{{
			PLMNIdentity:        plmn,
			TAISliceSupportList: support,
		}},
	}}

	return req, nil
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBDoesntSupportAnyTAC(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	// Use a valid TAC to satisfy the decoder (missing IE → ErrorIndication
	// at the dispatcher layer, not NGSetupFailure here), then erase the
	// item list to exercise the "gNB advertised no supported TAs" branch.
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

	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}, nil, nil)

	msg.SupportedTAList = nil

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure to be sent, but got %d", len(sender.SentNGSetupFailures))
	}

	cause := sender.SentNGSetupFailures[0].Cause
	if cause.Present != ngapType.CausePresentMisc {
		t.Fatalf("expected Cause Present to be Miscellaneous, but got %v", cause.Present)
	}

	if cause.Misc.Value != ngapType.CauseMiscPresentUnspecified {
		t.Errorf("expected Cause Miscellaneous Value to be CauseMiscPresentUnspecified, but got %v", cause.Misc.Value)
	}

	if ran.RanID != nil {
		t.Error("RanID should remain nil after failed NG Setup")
	}
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBSupportsDifferentTAC(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

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
	}

	err = op.SetSupportedTacs([]string{"000065", "000066"})
	if err != nil {
		t.Fatalf("failed to set supported TACS: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
	}, nil, nil)

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure to be sent, but got %d", len(sender.SentNGSetupFailures))
	}

	cause := sender.SentNGSetupFailures[0].Cause
	if cause.Present != ngapType.CausePresentMisc {
		t.Fatalf("expected Cause Present to be Miscellaneous, but got %v", cause.Present)
	}

	if cause.Misc.Value != ngapType.CauseMiscPresentUnspecified {
		t.Errorf("expected CauseMiscPresentUnspecified for TAC mismatch, got %v", cause.Misc.Value)
	}

	if ran.RanID != nil {
		t.Error("RanID should remain nil after failed NG Setup")
	}
}

func TestHandleNGSetupRequest_NGSetupResponse(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

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
	}

	err = op.SetSupportedTacs([]string{"000064", "000065"})
	if err != nil {
		t.Fatalf("failed to set supported TACS: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
	}, nil, nil)
	amfInstance.Name = "ella-core"

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse to be sent, but got %d", len(sender.SentNGSetupResponses))
	}

	response := sender.SentNGSetupResponses[0]

	if response.Guami == nil {
		t.Errorf("expected Guami to be set in NGSetupResponse, but it was nil")
	}

	if response.Guami.PlmnID.Mcc != "001" {
		t.Errorf("expected Guami PlmnID MCC to be '001', but got %s", response.Guami.PlmnID.Mcc)
	}

	if response.Guami.PlmnID.Mnc != "01" {
		t.Errorf("expected Guami PlmnID MNC to be '01', but got %s", response.Guami.PlmnID.Mnc)
	}

	if len(response.SnssaiList) != 1 {
		t.Fatalf("expected 1 slice in SnssaiList, got %d", len(response.SnssaiList))
	}

	if response.SnssaiList[0].Sst != 1 {
		t.Errorf("expected SnssaiList[0].Sst to be 1, got %d", response.SnssaiList[0].Sst)
	}

	if response.AmfName != "ella-core" {
		t.Errorf("expected AmfName to be 'ella-core', but got '%s'", response.AmfName)
	}

	if response.AmfRelativeCapacity != 0xff {
		t.Errorf("expected AmfRelativeCapacity to be 0xff, but got %d", response.AmfRelativeCapacity)
	}

	if ran.RanID == nil {
		t.Fatal("RanID should be set after successful NG Setup")
	}

	tais := amfInstance.RadioSupportedTAIsForTest(ran)
	if len(tais) != 1 {
		t.Fatalf("expected 1 SupportedTAI, got %d", len(tais))
	}

	if tais[0].Tai.Tac != "000064" {
		t.Errorf("expected TAC '000064', got %q", tais[0].Tai.Tac)
	}

	if len(tais[0].SNssaiList) != 1 {
		t.Fatalf("expected 1 SNssai in SupportedTAI, got %d", len(tais[0].SNssaiList))
	}

	if tais[0].SNssaiList[0].Sst != 1 {
		t.Errorf("expected SNssai SST 1, got %d", tais[0].SNssaiList[0].Sst)
	}

	if tais[0].SNssaiList[0].Sd != "010203" {
		t.Errorf("expected SNssai SD '010203', got %q", tais[0].SNssaiList[0].Sd)
	}
}

func TestHandleNGSetupRequest_MultipleSlicesInRequest(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Slices: []SliceOpt{
			{Sst: 1, Sd: "010203"},
			{Sst: 2, Sd: "aabbcc"},
			{Sst: 3, Sd: ""},
		},
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{
		Mcc: "001",
		Mnc: "01",
	}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
	}, nil, nil)
	amfInstance.Name = "ella-core"

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse, got %d", len(sender.SentNGSetupResponses))
	}

	tais := amfInstance.RadioSupportedTAIsForTest(ran)
	if len(tais) != 1 {
		t.Fatalf("expected 1 SupportedTAI, got %d", len(tais))
	}

	snssaiList := tais[0].SNssaiList
	if len(snssaiList) != 3 {
		t.Fatalf("expected 3 SNssai items in SupportedTAI, got %d", len(snssaiList))
	}

	expectedSlices := []struct {
		sst int32
		sd  string
	}{
		{1, "010203"},
		{2, "aabbcc"},
		{3, ""},
	}

	for i, expected := range expectedSlices {
		if snssaiList[i].Sst != expected.sst {
			t.Errorf("SNssai[%d]: expected SST %d, got %d", i, expected.sst, snssaiList[i].Sst)
		}

		if snssaiList[i].Sd != expected.sd {
			t.Errorf("SNssai[%d]: expected SD %q, got %q", i, expected.sd, snssaiList[i].Sd)
		}
	}
}

func TestHandleNGSetupRequest_ResponseContainsAllConfiguredSlices(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

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
	}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	sd1 := "010203"
	sd2 := "aabbcc"

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
		Slices: []db.NetworkSlice{
			{ID: "slice-1", Name: "eMBB", Sst: 1, Sd: &sd1},
			{ID: "slice-2", Name: "URLLC", Sst: 2, Sd: &sd2},
			{ID: "slice-3", Name: "mMTC", Sst: 3, Sd: nil},
		},
	}, nil, nil)
	amfInstance.Name = "ella-core"

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse, got %d", len(sender.SentNGSetupResponses))
	}

	response := sender.SentNGSetupResponses[0]

	if len(response.SnssaiList) != 3 {
		t.Fatalf("expected 3 slices in response SnssaiList, got %d", len(response.SnssaiList))
	}

	expectedSlices := []struct {
		sst int32
		sd  string
	}{
		{1, "010203"},
		{2, "aabbcc"},
		{3, ""},
	}

	for i, expected := range expectedSlices {
		if response.SnssaiList[i].Sst != expected.sst {
			t.Errorf("SnssaiList[%d]: expected SST %d, got %d", i, expected.sst, response.SnssaiList[i].Sst)
		}

		if response.SnssaiList[i].Sd != expected.sd {
			t.Errorf("SnssaiList[%d]: expected SD %q, got %q", i, expected.sd, response.SnssaiList[i].Sd)
		}
	}
}

func TestHandleNGSetupRequest_NGSetupFailure_PLMNMismatch(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "310",
		Mnc:   "410",
		Tac:   "000064",
		Sst:   1,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{Mcc: "001", Mnc: "01"}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{Operator: op}, nil, nil)

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure, got %d", len(sender.SentNGSetupFailures))
	}

	cause := sender.SentNGSetupFailures[0].Cause
	if cause.Present != ngapType.CausePresentMisc || cause.Misc.Value != ngapType.CauseMiscPresentUnknownPLMN {
		t.Errorf("expected UnknownPLMN cause for PLMN mismatch, got present=%d misc=%d", cause.Present, cause.Misc.Value)
	}
}

func TestHandleNGSetupRequest_DBFailure_SendsNGSetupFailure(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		OperatorErr: fmt.Errorf("database unavailable"),
	}, nil, nil)

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected NGSetupFailure on DB error, got %d", len(sender.SentNGSetupFailures))
	}

	cause := sender.SentNGSetupFailures[0].Cause
	if cause.Present != ngapType.CausePresentMisc || cause.Misc.Value != ngapType.CauseMiscPresentUnspecified {
		t.Errorf("expected Unspecified cause on DB failure, got present=%d misc=%d", cause.Present, cause.Misc.Value)
	}
}

func TestHandleNGSetupRequest_SliceDBFailure_SendsNGSetupFailure(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{Mcc: "001", Mnc: "01"}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator:  op,
		SlicesErr: fmt.Errorf("slice query failed"),
	}, nil, nil)

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected NGSetupFailure on slice DB error, got %d", len(sender.SentNGSetupFailures))
	}
}

func TestHandleNGSetupRequest_NoSliceOverlap_SucceedsWithWarning(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   2,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{Mcc: "001", Mnc: "01"}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
		Slices:   []db.NetworkSlice{{ID: "s1", Name: "eMBB", Sst: 1}},
	}, nil, nil)
	amfInstance.Name = "ella-core"

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected NGSetupResponse even with no slice overlap, got %d responses", len(sender.SentNGSetupResponses))
	}

	if len(sender.SentNGSetupFailures) != 0 {
		t.Errorf("expected no NGSetupFailure for slice mismatch, got %d", len(sender.SentNGSetupFailures))
	}
}
